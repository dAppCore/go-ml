# TODO.md — go-ml Task Queue

## Phase 1: go-inference Migration

The big one. `backend_mlx.go` needs rewriting to use `go-inference.TextModel` instead of direct go-mlx imports. This collapses ~253 LOC to ~60 LOC.

- [ ] **Rewrite backend_mlx.go** — Replace direct go-mlx calls with go-inference TextModel. The current implementation manually handles tokenisation, KV cache, sampling, and token decoding. go-inference wraps all of that behind `TextModel.Generate()` returning `iter.Seq[Token]`.
- [ ] **HTTPBackend go-inference wrapper** — HTTPBackend should implement `go-inference.Backend` or wrap it. Currently returns `(string, error)` from Generate; needs an adapter that yields `iter.Seq[Token]` from SSE streams.
- [ ] **LlamaBackend go-inference wrapper** — Same treatment as HTTPBackend. llama-server already supports SSE streaming; the adapter reads the stream and yields tokens.
- [ ] **Bridge ml.Backend to go-inference** — The old `ml.Backend` interface (`Generate` returns `string`, not `iter.Seq[Token]`) needs a bridging adapter. Write `InferenceAdapter` that wraps `go-inference.TextModel` and collects tokens into a string for the legacy interface.

## Phase 2: Backend Consolidation

`StreamingBackend` vs `go-inference.TextModel` overlap. Reconcile: go-inference is the standard, `ml.Backend` is legacy.

- [ ] **Audit StreamingBackend usage** — Find all callers of `GenerateStream`/`ChatStream`. Determine which can migrate directly to `iter.Seq[Token]`.
- [ ] **Migration path** — Keep both interfaces temporarily. Add `BackendAdapter` that wraps go-inference.TextModel and satisfies both `ml.Backend` and `StreamingBackend`.
- [ ] **Deprecate StreamingBackend** — Once all callers use go-inference iterators, mark StreamingBackend as deprecated. Remove in a later phase.
- [ ] **Unify GenOpts** — `ml.GenOpts` and `go-inference.GenerateOptions` likely overlap. Consolidate into one options struct or add conversion helpers.

## Phase 3: Agent Loop Modernisation

`agent.go` (1,070 LOC) is the largest file. SSH checkpoint discovery, InfluxDB streaming. Needs splitting into smaller files.

- [ ] **Split agent.go** — Decompose into: `agent_config.go` (SSH/infra config), `agent_execute.go` (scoring run orchestration), `agent_eval.go` (result evaluation and publishing), `agent_influx.go` (InfluxDB streaming).
- [ ] **Abstract SSH transport** — M3 homelab SSH may change to Linux. Extract SSH checkpoint discovery into an interface so the transport layer is swappable.
- [ ] **InfluxDB client modernisation** — Current line protocol writes are hand-rolled. Evaluate using the official InfluxDB Go client library.
- [ ] **Configurable endpoints** — Hardcoded `10.69.69.165:8181` and M3 SSH details should come from config/environment, not constants.

## Phase 4: Test Coverage

`backend_http_test` exists but `backend_llama` and `backend_mlx` have no tests. `score.go` concurrency needs race condition tests.

- [ ] **backend_llama_test.go** — Mock llama-server subprocess. Test: model loading, prompt formatting, streaming, error recovery, process lifecycle.
- [ ] **backend_mlx_test.go** — Mock go-mlx (or go-inference after Phase 1). Test: darwin/arm64 gating, Metal availability check, generation flow, tokeniser errors.
- [ ] **score.go race tests** — Run `go test -race ./...`. Add concurrent scoring tests: multiple suites running simultaneously, semaphore boundary conditions, context cancellation mid-score.
- [ ] **Benchmark suite** — Add `BenchmarkHeuristic`, `BenchmarkJudge`, `BenchmarkExact` for various input sizes. No benchmarks exist currently.

---

## Standing: Workflow

1. Virgil in core/go writes tasks here after research
2. This repo's session picks up tasks in phase order
3. Mark `[x]` when done, note commit hash
4. Phase 1 is the critical path — everything else builds on go-inference migration
