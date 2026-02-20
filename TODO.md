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

- [ ] **Audit StreamingBackend usage** — Find all callers of `GenerateStream`/`ChatStream`. Determine which can migrate to `iter.Seq[Token]`.
- [ ] **Deprecate StreamingBackend** — Once all callers use go-inference iterators, mark StreamingBackend as deprecated.
- [ ] **Unify GenOpts** — `ml.GenOpts` and `inference.GenerateConfig` overlap. Add `convertOpts()` in Phase 1, consolidate into one struct later.
- [ ] **Unify Message types** — `ml.Message` and `inference.Message` are identical structs. Consider type alias or shared import.

---

## Phase 3: Agent Loop Modernisation

`agent.go` (1,070 LOC) is the largest file. Decompose.

- [ ] **Split agent.go** — Into: `agent_config.go` (config, model maps), `agent_execute.go` (run loop, checkpoint processing), `agent_eval.go` (probe evaluation, result publishing), `agent_influx.go` (InfluxDB streaming, JSONL buffer).
- [ ] **Abstract SSH transport** — Extract SSH checkpoint discovery into interface. Current M3 homelab SSH may change to Linux (go-rocm).
- [ ] **Configurable endpoints** — `10.69.69.165:8181` and M3 SSH details hardcoded. Move to config/environment.
- [ ] **InfluxDB client** — Hand-rolled line protocol. Evaluate official InfluxDB Go client.

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
