# TODO.md — go-ml Task Queue

Dispatched from Virgil in core/go. Pick up tasks in phase order.

---

## Phase 1: go-inference Migration (CRITICAL PATH)

Everything downstream is blocked on this. The old `backend_mlx.go` imports go-mlx subpackages that no longer exist after Phase 4 refactoring.

### Step 1.1: Add go-inference dependency

- [x] **Add `forge.lthn.ai/core/go-inference` to go.mod** — Already has a `replace` directive pointing to `../go-inference`. Run `go get forge.lthn.ai/core/go-inference` then `go mod tidy`. Verify the module resolves.

### Step 1.2: Write the InferenceAdapter

- [x] **Create `adapter.go`** — Bridge between `go-inference.TextModel` (returns `iter.Seq[Token]`) and `ml.Backend` + `ml.StreamingBackend` (returns `string`/callback). Must implement:
  - `Generate()` — collect tokens from iterator into string
  - `Chat()` — same, using `TextModel.Chat()`
  - `GenerateStream()` — forward tokens to `TokenCallback`
  - `ChatStream()` — same for chat
  - `Name()` — delegate to `TextModel.ModelType()`
  - `Available()` — always true (model already loaded)
  - `convertOpts(GenOpts) []inference.GenerateOption` — map `GenOpts` fields to functional options

  **Key mapping**:
  ```
  GenOpts.Temperature → inference.WithTemperature(float32(t))
  GenOpts.MaxTokens   → inference.WithMaxTokens(n)
  GenOpts.Model       → (ignored, model already loaded)
  ```

  **Error handling**: After the iterator completes, check `model.Err()` to distinguish EOS from errors (OOM, ctx cancelled).

- [x] **Test adapter.go** — 13 test cases with mock TextModel (all pass). Test cases:
  - Normal generation (collect tokens → string)
  - Streaming (each token hits callback)
  - Callback error stops iteration
  - Context cancellation propagates
  - Empty output (EOS immediately)
  - Model error after partial output

### Step 1.3: Rewrite backend_mlx.go

- [x] **Replace backend_mlx.go** — Deleted the 253 LOC that manually handle tokenisation, KV cache, sampling, and memory cleanup. Replaced with ~35 LOC:
  ```go
  //go:build darwin && arm64

  package ml

  import (
      "forge.lthn.ai/core/go-inference"
      _ "forge.lthn.ai/core/go-mlx"  // registers "metal" backend
  )

  func NewMLXBackend(modelPath string) (*InferenceAdapter, error) {
      m, err := inference.LoadModel(modelPath)
      if err != nil {
          return nil, fmt.Errorf("mlx: %w", err)
      }
      return &InferenceAdapter{model: m, name: "mlx"}, nil
  }
  ```
  The `InferenceAdapter` from Step 1.2 handles all the Generate/Chat/Stream logic.

- [ ] **Preserve memory controls** — The old `MLXBackend` set cache/memory limits (16GB/24GB). Now delegated to go-mlx internally. Callers can still use `mlx.SetCacheLimit()`/`mlx.SetMemoryLimit()` directly. Options for future:
  - Accept memory limits in `NewMLXBackend` params
  - Or set them in `InferenceAdapter` wrapper
  - go-mlx exposes `SetCacheLimit()` / `SetMemoryLimit()` at package level

- [ ] **Test backend_mlx.go** — Verify the new backend can:
  - Load a model via go-inference registry
  - Generate text (smoke test, requires model on disk)
  - Stream tokens via callback
  - Handle Metal availability check (build tag gating)

### Step 1.4: HTTPBackend and LlamaBackend wrappers

- [x] **HTTPBackend go-inference wrapper** — `backend_http_textmodel.go`: `HTTPTextModel` wraps `HTTPBackend` to implement `inference.TextModel`. Generate/Chat yield entire response as single Token. Classify returns unsupported error. BatchGenerate processes prompts sequentially. 17 tests pass.

- [x] **LlamaBackend go-inference wrapper** — `backend_http_textmodel.go`: `LlamaTextModel` embeds `HTTPTextModel`, overrides `ModelType()` -> "llama" and `Close()` -> `llama.Stop()`. 2 tests pass.

### Step 1.5: Verify downstream consumers

- [x] **Service.Generate() still works** — `service.go` calls `Backend.Generate()`. InferenceAdapter satisfies ml.Backend. HTTPBackend/LlamaBackend still implement ml.Backend directly. No changes needed.
- [x] **Judge still works** — `judge.go` calls `Backend.Generate()` via `judgeChat()`. Same Backend contract, works as before. No changes needed.
- [x] **go-ai tools_ml.go** — Uses `ml.Service` directly. `ml.Backend` interface is preserved, no code changes needed in go-ai.

---

## Phase 2: Backend Consolidation

After Phase 1, both `ml.Backend` (string) and `inference.TextModel` (iterator) coexist. Reconcile.

### Audit Results (Virgil, 20 Feb 2026)

**StreamingBackend callers** — Only 2 files in `host-uk/cli`:
- `cmd/ml/cmd_serve.go` lines 146,201,319: Type-asserts `backend.(ml.StreamingBackend)` for SSE streaming at `/v1/completions` and `/v1/chat/completions`
- `cmd/ml/cmd_chat.go`: Direct `ChatStream()` call for interactive terminal token echo

All other consumers (service.go, judge.go, agent.go, expand.go, go-ai tools_ml.go) use `Backend.Generate()` — NOT streaming.

**Backend implementations**:
- `InferenceAdapter` → implements Backend + StreamingBackend (via go-inference iter.Seq)
- `HTTPBackend` → implements Backend only (no streaming)
- `LlamaBackend` → implements Backend only (no streaming)

### Step 2.1: Unify Message types

- [x] **Type alias ml.Message → inference.Message** — In `inference.go`, replace the `Message` struct with:
  ```go
  type Message = inference.Message
  ```
  This is backward-compatible — all existing callers keep working. Remove the `convertMessages()` helper from `adapter.go` since types are now identical. Verify with `go build ./...` and `go test ./...`.

### Step 2.2: Unify GenOpts

- [x] **Add inference fields to GenOpts** — Extend `ml.GenOpts` to include the extra fields from `inference.GenerateConfig`:
  ```go
  type GenOpts struct {
      Temperature   float64
      MaxTokens     int
      Model         string  // override model for this request
      TopK          int     // NEW: from inference.GenerateConfig
      TopP          float64 // NEW: from inference.GenerateConfig (float64 to match Temperature)
      RepeatPenalty float64 // NEW: from inference.GenerateConfig
  }
  ```
  Update `convertOpts()` in adapter.go to map the new fields. Existing callers that only set Temperature/MaxTokens/Model continue working unchanged.

### Step 2.3: Deprecate StreamingBackend

- [x] **Mark StreamingBackend as deprecated** — Add deprecation comment:
  ```go
  // Deprecated: StreamingBackend is retained for backward compatibility.
  // New code should use inference.TextModel with iter.Seq[Token] directly.
  // See InferenceAdapter for the bridge pattern.
  type StreamingBackend interface { ... }
  ```
  Do NOT remove yet — `host-uk/cli` cmd_serve.go and cmd_chat.go still depend on it. Those migrations are out of scope for go-ml (they live in a different repo).

### Step 2.4: Document migration path

- [x] **Update CLAUDE.md** — Add "Backend Architecture" section documenting:
  - `inference.TextModel` (iterator-based) is the preferred API for new code
  - `ml.Backend` (string-based) is the compatibility layer, still supported
  - `StreamingBackend` is deprecated, use `iter.Seq[Token]` directly
  - `InferenceAdapter` bridges TextModel → Backend/StreamingBackend
  - `HTTPTextModel`/`LlamaTextModel` bridges Backend → TextModel (reverse direction)

---

## Phase 3: Agent Loop Modernisation

`agent.go` (1,070 LOC) is the largest file with SSH, InfluxDB, scoring, and publishing mixed together. Decompose into focused files.

### Step 3.1: Split agent.go into 5 files — COMPLETE

- [x] **Split `agent.go` (1,070 LOC) into 5 focused files** — Commit `eae9ec9`. All `go build/test/vet` pass:
  - `agent_config.go` (97 LOC): AgentConfig, Checkpoint, BaseModelMap, ModelFamilies, AdapterMeta()
  - `agent_execute.go` (215 LOC): RunAgentLoop, DiscoverCheckpoints, GetScoredLabels, FindUnscored, ProcessOne, isMLXNative
  - `agent_eval.go` (397 LOC): processMLXNative, processWithConversion, RunCapabilityProbes/Full, RunContentProbes, ProbeResult types
  - `agent_influx.go` (291 LOC): ScoreCapabilityAndPush, ScoreContentAndPush, PushCapability*, BufferInfluxResult, ReplayInfluxBuffer
  - `agent_ssh.go` (102 LOC): SSHCommand, SCPFrom, SCPTo, fileBase, EnvOr, IntEnvOr, ExpandHome

### Step 3.2: Abstract SSH transport — COMPLETE

- [x] **RemoteTransport interface + SSHTransport** — Commit `1c2a6a6`. Interface with Run/CopyFrom/CopyTo, SSHTransport implementation with functional options (WithPort, WithTimeout). AgentConfig.Transport field with lazy init. All callers updated (DiscoverCheckpoints, processMLXNative, processWithConversion). Old SSHCommand/SCPFrom/SCPTo preserved as deprecated wrappers. Build/test/vet clean.

### Step 3.3: Configurable infrastructure — COMPLETE

- [x] **Extract hardcoded values to constants** — Commit `12f3a1c`. 15 constants in agent_config.go: EpochBase, 5 InfluxDB measurements, 2 DuckDB tables, probe defaults (temp/maxTokens/truncation), InfluxBufferFile, LogSeparatorWidth, InterCheckpointDelay. Hardcoded probe counts replaced with len(). 7 files, build/test/vet clean.

### Step 3.4: Agent tests

- [ ] **Test `AdapterMeta()`** — Extract model tag, label prefix, run ID from dirname patterns
- [ ] **Test `FindUnscored()`** — Filtering logic with mock scored labels
- [ ] **Test `BufferInfluxResult()`/`ReplayInfluxBuffer()`** — JSONL persistence round-trip
- [ ] **Test `DiscoverCheckpoints()`** — Mock SSH output parsing

---

## Phase 4: Test Coverage

- [ ] **backend_llama_test.go** — Mock llama-server subprocess. Test: model loading, health checks, process lifecycle.
- [ ] **backend_mlx_test.go** — After Phase 1 rewrite, test with mock go-inference TextModel.
- [ ] **score.go race tests** — `go test -race ./...`. Concurrent scoring, semaphore boundaries, context cancellation.
- [ ] **Benchmark suite** — `BenchmarkHeuristic`, `BenchmarkJudge`, `BenchmarkExact` for various input sizes.

---

## Workflow

1. Virgil in core/go writes tasks here after research
2. This repo's session picks up tasks in phase order
3. Mark `[x]` when done, note commit hash
4. New discoveries → add tasks, note in FINDINGS.md
5. Push to forge after each completed step: `git push forge main`
