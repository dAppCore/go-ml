# go-ml Project History

## Origin: Extraction from go-ai (19 February 2026)

go-ml began as the `ai/ml/` subpackage inside `forge.lthn.ai/core/go-ai`. The monolith had grown to approximately 14,000 LOC and 53% of that was the ML subsystem. The ML code had zero internal dependencies on the rest of go-ai — it imported only `go-mlx` (external) and the Core `go` framework. The extraction was therefore clean: lift the directory, adjust the module path, and update the one import in go-ai that referenced it.

**What was extracted:**

- 41 Go source files (~7,494 LOC, excluding tests)
- 6 test files covering backends, heuristic, judge, exact, probes, and score
- All InfluxDB, DuckDB, Parquet, GGUF, and agent code

**After extraction:**

- go-ai dropped from ~14,000 to ~3,400 LOC (the `ai/` facade and `mcp/` hub remain there)
- go-ml became an independent module at `dappco.re/go/core/ml`

---

## Phase 1: go-inference Migration (Complete)

**Commit range:** `c3c2c14` (initial fix) through adapter and reverse adapter work.

**Problem:** The original `backend_mlx.go` imported subpackages from go-mlx (`go-mlx/cache`, `go-mlx/model`, `go-mlx/sample`, `go-mlx/tokenizer`) that no longer existed after go-mlx's Phase 4 refactoring. The file was 253 LOC of hand-rolled tokenisation, KV cache management, sampling loops, and memory cleanup — and none of it compiled.

**Solution:** Introduce `go-inference` as the abstraction layer between go-ml and hardware backends.

### Step 1.1 — Add go-inference dependency

Added `forge.lthn.ai/core/go-inference` to `go.mod` with a `replace` directive pointing to the local sibling checkout.

### Step 1.2 — Write InferenceAdapter (`adapter.go`)

Created `InferenceAdapter`, which wraps a `go-inference.TextModel` (returning `iter.Seq[Token]`) and exposes it as `ml.Backend` + `ml.StreamingBackend` (returning strings / calling `TokenCallback`). Thirteen test cases verified token collection, streaming, callback error propagation, context cancellation, empty output, and model errors after partial generation.

Key design decision: after exhausting the iterator, `model.Err()` is checked separately. The iterator itself does not carry errors; partial output is returned alongside the error so callers can decide whether to use or discard it.

### Step 1.3 — Rewrite `backend_mlx.go`

Replaced 253 LOC with approximately 35 LOC. The blank import `_ "dappco.re/go/core/mlx"` registers the Metal backend via go-mlx's `init()`. `inference.LoadModel()` then handles model loading, and `InferenceAdapter` handles the rest.

Memory controls (cache limits, memory limits) were deferred: go-mlx handles them internally, and callers that need explicit control can call `mlx.SetCacheLimit()` directly.

### Step 1.4 — Reverse adapters (`backend_http_textmodel.go`)

Added `HTTPTextModel` and `LlamaTextModel`, which wrap the existing `ml.Backend` implementations to satisfy `inference.TextModel`. This enables HTTP and llama-server backends to be used in packages (go-ai, go-i18n) that consume the go-inference interface. Since HTTP backends return complete strings rather than streaming tokens, each response is yielded as a single `Token`.

17 tests for `HTTPTextModel` and 2 for `LlamaTextModel` all pass.

### Step 1.5 — Downstream verification

Confirmed that `service.go` (`Backend.Generate()`), `judge.go` (`judgeChat()`), and `go-ai/mcp/tools_ml.go` (`ml.Service`) required no changes — `InferenceAdapter` satisfies `ml.Backend`, and the existing consumers are unaffected.

---

## Phase 2: Backend Consolidation (Complete)

**Commit range:** `747e703` (Message unification) through `convertOpts` extension.

**Audit (Virgil, 20 February 2026):** Only two files in the entire ecosystem call `StreamingBackend` methods: `host-uk/cli/cmd/ml/cmd_serve.go` (SSE streaming at `/v1/completions` and `/v1/chat/completions`) and `cmd/ml/cmd_chat.go` (interactive terminal token echo). All other consumers use `Backend.Generate()` only.

### Step 2.1 — Unify Message types

`ml.Message` was a separate struct identical to `inference.Message`. Replaced with a type alias:

```go
type Message = inference.Message
```

This eliminated the `convertMessages()` helper from `adapter.go` and all explicit conversion sites. Backward-compatible: all existing callers continue to use `ml.Message` and compile unchanged.

### Step 2.2 — Extend GenOpts

Added `TopK`, `TopP`, and `RepeatPenalty` to `ml.GenOpts` to match the fields available in `inference.GenerateConfig`. Updated `convertOpts()` in `adapter.go` to map the new fields. Existing callers that only set `Temperature`, `MaxTokens`, and `Model` continue to work unchanged.

**Field type note:** `inference.GenerateConfig` uses `float32` for temperature and sampling fields; `ml.GenOpts` uses `float64` to match the conventions in the rest of go-ml. `convertOpts()` performs the narrowing conversion explicitly.

### Step 2.3 — Deprecate StreamingBackend

Added deprecation comment to `StreamingBackend` in `inference.go`. The interface is not removed because `host-uk/cli` depends on it. Migration of those CLI files is out of scope for go-ml.

### Step 2.4 — Document backend architecture

Added the "Backend Architecture" section to `CLAUDE.md` documenting the two interface families, adapter directions, and migration guidance.

---

## Phase 3: Agent Loop Modernisation (Complete)

The original `agent.go` was a 1,070 LOC file mixing SSH commands, InfluxDB line protocol construction, probe evaluation, checkpoint discovery, and JSONL buffering. It had zero tests.

### Step 3.1 — Split into five files (Commit `eae9ec9`)

| File | LOC | Contents |
|------|-----|---------|
| `agent_config.go` | 97 | `AgentConfig`, `Checkpoint`, `BaseModelMap`, `ModelFamilies`, `AdapterMeta()` |
| `agent_execute.go` | 215 | `RunAgentLoop`, `DiscoverCheckpoints`, `GetScoredLabels`, `FindUnscored`, `ProcessOne`, `isMLXNative` |
| `agent_eval.go` | 397 | `processMLXNative`, `processWithConversion`, `RunCapabilityProbes`, `RunCapabilityProbesFull`, `RunContentProbes`, `ProbeResult` types |
| `agent_influx.go` | 291 | `ScoreCapabilityAndPush`, `ScoreContentAndPush`, `PushCapability*`, `BufferInfluxResult`, `ReplayInfluxBuffer` |
| `agent_ssh.go` | 102 | `SSHCommand`, `SCPFrom`, `SCPTo`, `fileBase`, `EnvOr`, `IntEnvOr`, `ExpandHome` |

`go build ./...`, `go test ./...`, and `go vet ./...` all passed after the split.

### Step 3.2 — Abstract SSH transport (Commit `1c2a6a6`)

Introduced the `RemoteTransport` interface with `Run`, `CopyFrom`, and `CopyTo` methods. `SSHTransport` implements this interface using the system `ssh` and `scp` binaries with functional options (`WithPort`, `WithTimeout`). `AgentConfig.Transport` accepts any `RemoteTransport`, with lazy initialisation to an `SSHTransport` when nil.

The old package-level functions `SSHCommand`, `SCPFrom`, and `SCPTo` are retained as deprecated wrappers that delegate to `AgentConfig.Transport`.

### Step 3.3 — Extract hardcoded infrastructure (Commit `12f3a1c`)

Extracted 15 constants from scattered magic values across 7 files:

- `EpochBase` — InfluxDB timestamp origin (Unix timestamp for 15 February 2025 00:00 UTC)
- Five InfluxDB measurement names (`MeasurementCapabilityScore`, `MeasurementCapabilityJudge`, `MeasurementContentScore`, `MeasurementProbeScore`, `MeasurementTrainingLoss`)
- Two DuckDB table names (`TableCheckpointScores`, `TableProbeResults`)
- Probe evaluation defaults (`CapabilityTemperature`, `CapabilityMaxTokens`, `ContentTemperature`, `ContentMaxTokens`, `MaxStoredResponseLen`)
- `InfluxBufferFile` — JSONL buffer filename
- `LogSeparatorWidth` — banner line width

Hardcoded probe counts replaced with `len(CapabilityProbes)` and `len(ContentProbes)`.

### Step 3.4 — Agent tests (Commit `3e22761`)

First test coverage for the agent subsystem:

- `AdapterMeta()` — 8 tests: known families (12 entries), variant suffixes, subdirectory patterns, unknown fallback, no-prefix edge case
- `FindUnscored()` — 5 tests: all unscored (sorted), some scored, all scored, empty input, nil scored map
- `BufferInfluxResult()`/`ReplayInfluxBuffer()` — 4 tests: JSONL round-trip, multiple entries, empty file, missing file
- `DiscoverCheckpoints()` — 6 tests using `fakeTransport`: 3 checkpoints across 2 dirs, subdirectory pattern, no adapters, SSH error, filter pattern, no safetensors files

---

## Phase 4: Test Coverage (Complete, Commit `09bf403`)

Added four test files covering previously untested areas:

**`backend_llama_test.go`** (20 tests) — Uses `net/http/httptest` to mock the llama-server HTTP API. Covers: `Name`, `Available` (4 variants including process-not-started and health endpoint failure), `Generate` (6 variants including context cancellation, empty choices, and opts forwarding), `Chat` (3 variants), `Stop`, constructor (4 variants), and interface compliance.

**`backend_mlx_test.go`** (8 tests) — Uses a mock `inference.TextModel`. No build tag required — tests run on all platforms without Metal GPU hardware. Covers: `Generate`, `Chat`, streaming, model error after partial output, `Close`, direct model access via `Model()`, interface compliance, and `convertOpts` field mapping.

**`score_race_test.go`** (6 tests) — Race condition tests run with `-race`:
- `ConcurrentSemantic` — 20 responses scored with concurrency=4; verifies no data races on the result map
- `ConcurrentMixedSuites` — semantic + standard + content fan-out simultaneously
- `SemaphoreBoundary` — concurrency=1; verifies that at most 1 goroutine holds the semaphore at once
- `ContextCancellation` — 400 error response from judge returns nil semantic score without panicking
- `HeuristicOnlyNoRace` — 50 responses, heuristic only (no goroutines spawned); regression check
- `MultiModelConcurrent` — 4 models × 5 concurrent goroutines writing to the results map

**`benchmark_test.go`** (25 benchmarks, baselines on M3 Ultra):
- `HeuristicScore` — 5 input sizes (100–10,000 characters): 25µs–8.8ms
- `ExactMatch` — 4 patterns: 171ns–2.1µs
- `JudgeExtractJSON` — 6 response variants: 2.5–3.4µs
- `Judge` round-trip — 2 suites (semantic, content): ~52µs
- `ScoreAll` — 2 modes (heuristic only, full): 25µs–4.5ms
- Sub-components — 5 heuristic stages: 244ns–88µs

---

## Known Limitations

### StreamingBackend retention

`ml.StreamingBackend` cannot be removed until `host-uk/cli/cmd/ml/cmd_serve.go` and `cmd/ml/cmd_chat.go` are migrated to use `inference.TextModel` iterators directly. That migration is out of scope for go-ml and must be tracked in the `host-uk/cli` repository.

### LlamaTextModel streaming gap

`LlamaTextModel` implements `inference.TextModel` but does not actually stream tokens — it yields the complete llama-server HTTP response as a single `Token`. True token-level streaming from llama-server would require implementing SSE parsing, which is a separate effort.

### Agent infrastructure coupling

`AgentConfig` contains fields (`M3Host`, `M3User`, `M3SSHKey`, `M3AdapterBase`, `InfluxURL`, `InfluxDB`) that are tightly coupled to a specific deployment topology (M3 Mac + InfluxDB on `10.69.69.165`). While the `RemoteTransport` abstraction decouples tests from SSH, production deployments still hardcode the M3 as the checkpoint host.

### EpochBase timestamp

The `EpochBase` constant (`1739577600`, corresponding to 15 February 2025 00:00 UTC) is embedded in InfluxDB line protocol timestamps. All capability/content/probe timestamps derive from this base plus checkpoint iteration offsets. Changing `EpochBase` would require re-writing all historical InfluxDB data.

### HTTPBackend classify

`HTTPTextModel.Classify` returns an "unsupported" error. There is no path to add classification support to an OpenAI-compatible HTTP backend without a dedicated classification endpoint or prompt engineering.

### DuckDB CGo

The `go-duckdb` dependency requires CGo. This prevents cross-compilation from macOS to Linux without a cross-compilation toolchain. Binaries that import go-ml will require a C compiler at build time.

---

## Future Considerations

- **ROCm backend** — `go-rocm` provides a llama-server subprocess backend for AMD GPUs. Once published, it can be wrapped with `InferenceAdapter` in the same pattern as `backend_mlx.go`, gated with a `//go:build linux && amd64` constraint.
- **StreamingBackend removal** — Once `host-uk/cli` is migrated to `iter.Seq[Token]`, the `StreamingBackend` interface and `InferenceAdapter`'s `GenerateStream`/`ChatStream` methods can be removed.
- **go-i18n integration** — go-i18n Phase 2a requires 5,000 sentences/second classification throughput from Gemma3-1B. The `InferenceAdapter` and `inference.TextModel.BatchGenerate` provide the interface; the performance target depends on go-mlx's batching implementation.
- **LEM Lab pipeline wiring** — Integration tests for `backend_mlx.go` with a real model are deferred until the LEM Lab inference pipeline is fully wired. A smoke test against a small quantised model would confirm end-to-end Metal GPU inference through the go-inference abstraction.
- **Charm SSH** — The `SSHTransport` currently shells out to the system `ssh` and `scp` binaries. Replacing these with pure-Go SSH via `charmbracelet/keygen` and a native SSH client would eliminate the subprocess dependency and improve testability.
