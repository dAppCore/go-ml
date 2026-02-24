# go-ml Backend Result Type Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Break the Backend interface to return `Result{Text, Metrics}` instead of bare `string`, giving all consumers access to inference metrics.

**Architecture:** Add a `Result` struct to `inference.go`, update `Backend` and `StreamingBackend` interfaces, update all 3 backend implementations (HTTP, Llama, InferenceAdapter), then update ~13 production call sites and ~15 test call sites. The InferenceAdapter populates `Metrics` from the underlying TextModel; HTTP and Llama return nil metrics.

**Tech Stack:** Go 1.25, `forge.lthn.ai/core/go-inference` (GenerateMetrics type), testify

**Test command:** `go test ./...` (no Taskfile — standard go test)

**Build tags:** Several files are `//go:build darwin && arm64` (MLX-specific). On macOS arm64 all tests run. On other platforms, only HTTP backend tests run.

---

### Task 1: Add Result type and update Backend interface

**Files:**
- Modify: `inference.go:23-66`

**Step 1: Add the Result struct and update interfaces**

In `inference.go`, add the `Result` type after the imports and before the `Backend` interface. Then change `Backend.Generate` and `Backend.Chat` return types from `(string, error)` to `(Result, error)`:

```go
// Result holds the response text and optional inference metrics.
// Backends that support metrics (e.g. MLX via InferenceAdapter) populate
// Metrics; HTTP and subprocess backends leave it nil.
type Result struct {
	Text    string
	Metrics *inference.GenerateMetrics
}

// Backend is the primary inference abstraction. All three concrete
// implementations — HTTPBackend, LlamaBackend, InferenceAdapter — satisfy it.
type Backend interface {
	// Generate sends a single user prompt and returns the response.
	Generate(ctx context.Context, prompt string, opts GenOpts) (Result, error)

	// Chat sends a multi-turn conversation and returns the response.
	Chat(ctx context.Context, messages []Message, opts GenOpts) (Result, error)

	// Name returns the backend identifier (e.g. "http", "llama", "ollama").
	Name() string

	// Available reports whether the backend is ready to accept requests.
	Available() bool
}
```

`StreamingBackend` stays unchanged — it uses callbacks, not return values.

**Step 2: Verify the build fails**

Run: `go build ./...`
Expected: Compilation errors in every file that implements or calls Backend.Generate/Chat.

**Step 3: Commit**

```bash
git add inference.go
git commit -m "feat: add Result type, break Backend interface to return Result

Backend.Generate and Backend.Chat now return (Result, error) instead of
(string, error). Result carries the response text and optional
inference.GenerateMetrics for backends that support them.

Co-Authored-By: Virgil <virgil@lethean.io>"
```

---

### Task 2: Update InferenceAdapter (Metal backend)

**Files:**
- Modify: `adapter.go:33-57`

**Step 1: Update Generate and Chat to return Result with Metrics**

```go
// Generate collects all tokens from the model's iterator into a single string.
func (a *InferenceAdapter) Generate(ctx context.Context, prompt string, opts GenOpts) (Result, error) {
	inferOpts := convertOpts(opts)
	var b strings.Builder
	for tok := range a.model.Generate(ctx, prompt, inferOpts...) {
		b.WriteString(tok.Text)
	}
	if err := a.model.Err(); err != nil {
		return Result{Text: b.String()}, err
	}
	return Result{Text: b.String(), Metrics: metricsPtr(a.model)}, nil
}

// Chat sends a multi-turn conversation to the underlying TextModel and collects
// all tokens.
func (a *InferenceAdapter) Chat(ctx context.Context, messages []Message, opts GenOpts) (Result, error) {
	inferOpts := convertOpts(opts)
	var b strings.Builder
	for tok := range a.model.Chat(ctx, messages, inferOpts...) {
		b.WriteString(tok.Text)
	}
	if err := a.model.Err(); err != nil {
		return Result{Text: b.String()}, err
	}
	return Result{Text: b.String(), Metrics: metricsPtr(a.model)}, nil
}
```

Add a helper at the bottom of adapter.go:

```go
// metricsPtr returns a copy of the model's latest metrics, or nil if unavailable.
func metricsPtr(m inference.TextModel) *inference.GenerateMetrics {
	met := m.Metrics()
	return &met
}
```

**Step 2: Verify adapter compiles**

Run: `go build ./...`
Expected: Still fails (other backends + callers not updated yet), but `adapter.go` should have no errors.

**Step 3: Commit**

```bash
git add adapter.go
git commit -m "feat(adapter): return Result with Metrics from TextModel

InferenceAdapter.Generate and Chat now return Result{Text, Metrics}
where Metrics is populated from the underlying TextModel.Metrics().

Co-Authored-By: Virgil <virgil@lethean.io>"
```

---

### Task 3: Update HTTPBackend

**Files:**
- Modify: `backend_http.go:77-128`

**Step 1: Update Generate and Chat return types**

```go
// Generate sends a single prompt and returns the response.
func (b *HTTPBackend) Generate(ctx context.Context, prompt string, opts GenOpts) (Result, error) {
	return b.Chat(ctx, []Message{{Role: "user", Content: prompt}}, opts)
}

// Chat sends a multi-turn conversation and returns the response.
func (b *HTTPBackend) Chat(ctx context.Context, messages []Message, opts GenOpts) (Result, error) {
	// ... existing code unchanged until the return statements ...
```

In `Chat`, change the success return in the retry loop (line ~117):
```go
		result, err := b.doRequest(ctx, body)
		if err == nil {
			return Result{Text: result}, nil
		}
```

Change the final error return (line ~127):
```go
	return Result{}, log.E("ml.HTTPBackend.Chat", fmt.Sprintf("exhausted %d retries", maxAttempts), lastErr)
```

Also update `doRequest` — it currently returns `(string, error)`. Keep it returning string since it's internal, or update it too. Simplest: keep `doRequest` as `(string, error)` since it's only called by `Chat`.

**Step 2: Commit**

```bash
git add backend_http.go
git commit -m "feat(http): return Result from Generate/Chat

HTTP backend returns Result{Text: text} with nil Metrics since
remote APIs don't provide Metal-level inference metrics.

Co-Authored-By: Virgil <virgil@lethean.io>"
```

---

### Task 4: Update LlamaBackend

**Files:**
- Modify: `backend_llama.go:118-130`

**Step 1: Update Generate and Chat**

LlamaBackend delegates to `b.http` (an HTTPBackend). Since HTTPBackend now returns Result, just update the signatures:

```go
// Generate delegates to the HTTP backend.
func (b *LlamaBackend) Generate(ctx context.Context, prompt string, opts GenOpts) (Result, error) {
	return b.http.Generate(ctx, prompt, opts)
}

// Chat delegates to the HTTP backend.
func (b *LlamaBackend) Chat(ctx context.Context, messages []Message, opts GenOpts) (Result, error) {
	return b.http.Chat(ctx, messages, opts)
}
```

**Step 2: Commit**

```bash
git add backend_llama.go
git commit -m "feat(llama): return Result from Generate/Chat

Delegates to HTTPBackend which already returns Result.

Co-Authored-By: Virgil <virgil@lethean.io>"
```

---

### Task 5: Update HTTPBackendTextModel

**Files:**
- Modify: `backend_http_textmodel.go:40-65`

**Step 1: Update the TextModel wrapper**

This file wraps HTTPBackend as a go-inference TextModel. It calls `m.http.Generate()` and `m.http.Chat()` internally. Update to access `.Text`:

Line ~42:
```go
		result, err := m.http.Generate(ctx, prompt, genOpts)
		if err != nil {
			// ... existing error handling
		}
		// Use result.Text where the old code used result directly
```

Line ~64:
```go
		result, err := m.http.Chat(ctx, messages, genOpts)
		if err != nil {
			// ... existing error handling
		}
		// Use result.Text where the old code used result directly
```

**Step 2: Commit**

```bash
git add backend_http_textmodel.go
git commit -m "refactor(http-textmodel): unwrap Result.Text from Backend calls

Co-Authored-By: Virgil <virgil@lethean.io>"
```

---

### Task 6: Update service.go facade

**Files:**
- Modify: `service.go:144-154`

**Step 1: Update Service.Generate return type**

```go
// Generate generates text using the named backend (or default).
func (s *Service) Generate(ctx context.Context, backendName, prompt string, opts GenOpts) (Result, error) {
	b := s.Backend(backendName)
	if b == nil {
		b = s.DefaultBackend()
	}
	if b == nil {
		return Result{}, fmt.Errorf("no backend available (requested: %q)", backendName)
	}
	return b.Generate(ctx, prompt, opts)
}
```

**Step 2: Commit**

```bash
git add service.go
git commit -m "refactor(service): Generate returns Result

Co-Authored-By: Virgil <virgil@lethean.io>"
```

---

### Task 7: Update production callers — root package

**Files:**
- Modify: `expand.go:103`
- Modify: `judge.go:62-63`
- Modify: `agent_eval.go:219,279,339`

**Step 1: Update expand.go**

Line 103 — add `.Text`:
```go
result, err := backend.Generate(ctx, p.Prompt, GenOpts{Temperature: 0.7, MaxTokens: 2048})
// ... error handling unchanged ...
response := result.Text
```

Rename the variable from `response` to `result` and add `response := result.Text` after error check. Or simply:
```go
res, err := backend.Generate(ctx, p.Prompt, GenOpts{Temperature: 0.7, MaxTokens: 2048})
if err != nil {
    // ... unchanged
}
response := res.Text
```

**Step 2: Update judge.go**

Line 62-63 — `judgeChat` returns `(string, error)` to its callers. Unwrap internally:
```go
func (j *Judge) judgeChat(ctx context.Context, prompt string) (string, error) {
	res, err := j.backend.Generate(ctx, prompt, DefaultGenOpts())
	return res.Text, err
}
```

**Step 3: Update agent_eval.go**

Three call sites. Each currently does `response, err := backend.Generate(...)`. Change to:

Line 219 (`RunCapabilityProbes`):
```go
res, err := backend.Generate(ctx, probe.Prompt, GenOpts{Temperature: CapabilityTemperature, MaxTokens: CapabilityMaxTokens})
// ... error handling unchanged (uses err) ...
response := res.Text
```

Note: line 222 uses `response` in error path — on error, `res.Text` will be empty string which is fine. But check: the existing code at line 282 does `response = fmt.Sprintf("ERROR: %v", err)` on error. This pattern needs `response` to be reassignable. Use:
```go
res, err := backend.Generate(...)
response := res.Text
if err != nil {
    response = fmt.Sprintf("ERROR: %v", err)
}
```

Line 279 (`RunCapabilityProbesFull`) — same error-path pattern as above. Note: downstream uses of `response` include `StripThinkBlocks(response)` at line 285, `fullResponses` append at lines 305-313, and `onProbe` callback at line 321. All of these expect a string, which is satisfied by extracting `response := res.Text` at the call site.

Line 339 (`RunContentProbesViaAPI`):
```go
res, err := backend.Generate(ctx, probe.Prompt, GenOpts{Temperature: ContentTemperature, MaxTokens: ContentMaxTokens})
if err != nil {
    // ... unchanged
}
reply := res.Text
```

**Step 4: Commit**

```bash
git add expand.go judge.go agent_eval.go
git commit -m "refactor: unwrap Result.Text in expand, judge, agent_eval

Co-Authored-By: Virgil <virgil@lethean.io>"
```

---

### Task 8: Update production callers — cmd/ package

**Files:**
- Modify: `cmd/cmd_ab.go:252,275`
- Modify: `cmd/cmd_sandwich.go:175`
- Modify: `cmd/cmd_lesson.go:245`
- Modify: `cmd/cmd_sequence.go:257`
- Modify: `cmd/cmd_benchmark.go:234,264`
- Modify: `cmd/cmd_serve.go:250,380`

**Step 1: Update cmd_ab.go**

**Important:** `baseResp` and `resp` are used as strings throughout the loop body — passed to `ml.ScoreHeuristic(baseResp)`, stored in `abConditionScore{Response: baseResp}`, used in `len(baseResp)`, and logged in `slog.Info`. Extract `.Text` at the call site so the existing string variable name is preserved for all downstream uses.

Line 252 — baseline response:
```go
res, err := backend.Chat(context.Background(), []ml.Message{
    {Role: "user", Content: p.Prompt},
}, opts)
if err != nil {
    slog.Error("ab: baseline failed", "id", p.ID, "error", err)
    runtime.GC()
    continue
}
baseResp := res.Text
```

Line 275 — kernel condition:
```go
res, err := backend.Chat(context.Background(), []ml.Message{
    {Role: "system", Content: k.Text},
    {Role: "user", Content: p.Prompt},
}, opts)
if err != nil {
    slog.Error("ab: failed", "id", p.ID, "condition", k.Name, "error", err)
    continue
}
resp := res.Text
```

**Step 2: Update cmd_sandwich.go**

Line 175:
```go
res, err := backend.Chat(context.Background(), messages, opts)
if err != nil {
    // ... unchanged
}
response := res.Text
```

**Step 3: Update cmd_lesson.go**

Line 245 — same pattern as sandwich:
```go
res, err := backend.Chat(context.Background(), messages, opts)
if err != nil {
    // ... unchanged
}
response := res.Text
```

**Step 4: Update cmd_sequence.go**

Line 257:
```go
res, err := backend.Chat(cmd.Context(), messages, opts)
if err != nil {
    // ... unchanged
}
response := res.Text
```

**Step 5: Update cmd_benchmark.go**

Line 234:
```go
res, err := baselineBackend.Generate(context.Background(), p.prompt, opts)
// ... unwrap res.Text ...
resp := res.Text
```

Line 264:
```go
res, err := trainedBackend.Generate(context.Background(), p.prompt, opts)
resp := res.Text
```

**Step 6: Update cmd_serve.go**

Line 250 (completions endpoint):
```go
res, err := backend.Generate(r.Context(), req.Prompt, opts)
if err != nil {
    // ... unchanged
}
text := res.Text
```

Line 380 (chat completions, non-streaming):
```go
res, err := backend.Chat(r.Context(), req.Messages, opts)
if err != nil {
    // ... unchanged
}
text := res.Text
```

**Step 7: Update api/routes.go**

Line 125 — note that the `text` variable is also used on line 131 in `generateResponse{Text: text}`. Use `res.Text` consistently:
```go
res, err := r.service.Generate(c.Request.Context(), req.Backend, req.Prompt, opts)
if err != nil {
    // ... unchanged
}
// line 131: generateResponse{Text: res.Text}
```

Either extract `text := res.Text` and use `text` on line 131, or use `res.Text` directly in the response struct. Both work — just be consistent.

**Step 8: Commit**

```bash
git add cmd/ api/
git commit -m "refactor(cmd): unwrap Result.Text across all commands

Updates cmd_ab, cmd_sandwich, cmd_lesson, cmd_sequence,
cmd_benchmark, cmd_serve, and api/routes.

Co-Authored-By: Virgil <virgil@lethean.io>"
```

---

### Task 9: Update all test files

**Files:**
- Modify: `adapter_test.go`
- Modify: `backend_http_test.go`
- Modify: `backend_llama_test.go`
- Modify: `backend_mlx_test.go`
- Modify: `backend_http_textmodel_test.go`
- No change: `api/routes_test.go` — tests use nil service and never reach Generate calls. Confirm by grepping for `.Generate` in that file.

**Step 1: Update adapter_test.go**

Every `result, err := adapter.Generate(...)` or `adapter.Chat(...)` — the `result` is now a `Result` struct. Add `.Text` to assertions:

```go
// Before:
result, err := adapter.Generate(context.Background(), "prompt", GenOpts{})
assert.Equal(t, "hello world", result)

// After:
result, err := adapter.Generate(context.Background(), "prompt", GenOpts{})
assert.Equal(t, "hello world", result.Text)
```

Also add a metrics assertion for the happy path:
```go
assert.NotNil(t, result.Metrics)
```

For error cases where `model.Err()` returns an error, `result.Text` may be partial and `Metrics` will be nil.

**Step 2: Update backend_http_test.go**

Same pattern — `result` → `result.Text` in assertions. Metrics will be nil for HTTP backend:
```go
result, err := b.Generate(context.Background(), "hello", DefaultGenOpts())
require.NoError(t, err)
assert.Equal(t, "test response", result.Text)
assert.Nil(t, result.Metrics)
```

**Step 3: Update backend_llama_test.go**

Same pattern as HTTP tests. `result.Text` everywhere.

**Step 4: Update backend_mlx_test.go**

Same pattern. Can also assert `result.Metrics != nil` on success.

**Step 5: Update backend_http_textmodel_test.go**

This tests the TextModel wrapper — it calls `model.Generate()` which returns `iter.Seq[Token]`, not `Backend.Generate()`. These tests likely don't need changes unless they also test the Backend interface directly. Check carefully.

**Step 6: Run all tests**

Run: `go test ./...`
Expected: All tests pass.

**Step 7: Commit**

```bash
git add *_test.go
git commit -m "test: update all test assertions for Result type

All Backend.Generate/Chat calls now return Result. Test assertions
updated to use .Text and check .Metrics where appropriate.

Co-Authored-By: Virgil <virgil@lethean.io>"
```

---

### Task 10: Final verification

**Step 1: Full build**

Run: `go build ./...`
Expected: Clean build, zero errors.

**Step 2: Full test suite**

Run: `go test ./... -count=1`
Expected: All tests pass.

**Step 3: Vet**

Run: `go vet ./...`
Expected: No issues.

**Step 4: Check for any remaining string returns**

Search for any callers still expecting `(string, error)` from Backend:

Run: `grep -rn '\.Generate\|\.Chat' --include='*.go' | grep -v '_test.go' | grep -v '//go:build ignore'`

Verify no call sites were missed.

**Step 5: Final commit if any fixups needed, then tag**

```bash
git tag -a v0.X.0 -m "feat: Backend returns Result{Text, Metrics}"
```
