# FINDINGS.md — go-ml Research & Discovery

## 2026-02-19: Split from go-ai (Virgil)

### Origin

Split from go-ai on 19 Feb 2026. Was `ai/ml/` subpackage inside `forge.lthn.ai/core/go-ai`. Zero internal go-ai dependencies — imports go-mlx (external module) and core/go framework only.

### What Was Extracted

- 41 Go files (~7,494 LOC excluding tests)
- 6 test files (backend_http, exact, heuristic, judge, probes, score)
- ml/ was 53% of go-ai's total LOC. After extraction, go-ai drops from ~14K to ~3.4K LOC (ai/ facade + mcp/ hub).

### Dependencies

- `forge.lthn.ai/core/go-mlx` — Metal GPU inference (backend_mlx.go, darwin/arm64 only)
- `forge.lthn.ai/core/go` — Framework services, process management, logging
- `github.com/marcboeker/go-duckdb` — Analytics storage
- `github.com/parquet-go/parquet-go` — Columnar data I/O
- `github.com/stretchr/testify` — Test assertions

### Consumers

- `go-ai/mcp/tools_ml.go` — Exposes ML as MCP tools
- `go-ai/test-mlx.go` — Integration test utility
- LEM Lab — Uses MLXBackend for chat inference

## Architecture

### Backend Interface

```go
type Backend interface {
    Generate(ctx context.Context, prompt string, opts GenOpts) (string, error)
    Chat(ctx context.Context, messages []Message, opts GenOpts) (string, error)
    Name() string
    Available() bool
}

type StreamingBackend interface {
    Backend
    GenerateStream(ctx context.Context, prompt string, opts GenOpts, cb TokenCallback) error
    ChatStream(ctx context.Context, messages []Message, opts GenOpts, cb TokenCallback) error
}
```

Key design: `Backend.Generate` returns `string`, not `iter.Seq[Token]`. `StreamingBackend` adds token callbacks but is still callback-based, not iterator-based.

### Scoring Engine

Concurrent scoring with semaphore-bounded workers. `Engine` fans out suites across goroutines, collects results.

**Heuristic suite** (9 metrics): refusal detection, length ratio, repetition, coherence, instruction following, format compliance, language match, confidence calibration, response diversity.

**Semantic suite** (4 dimensions): LLM-as-judge scoring across helpfulness, accuracy, harmlessness, and reasoning quality.

**Content suite** (6 probes): sovereignty probes testing model behaviour on sensitive topics — political bias, cultural sensitivity, factual grounding, source attribution, opinion vs fact distinction, regional awareness.

**Standard suite** (4 benchmarks): TruthfulQA (truthfulness), DoNotAnswer (safety refusals), Toxigen (toxicity detection), GSM8K (mathematical reasoning).

**Exact suite** (GSM8K numeric): Extracts numeric answers from model output and compares against ground truth with tolerance.

### 23 Capability Probes

16 categories covering: reasoning, mathematics, coding, instruction following, multilingual, summarisation, creative writing, factual recall, safety, ethics, roleplay, context length, tool use, multimodal description, structured output, and chain-of-thought.

### InfluxDB Integration

- Endpoint: `10.69.69.165:8181`
- Database: `training`
- Protocol: Line protocol writes (hand-rolled, no official client)
- Purpose: Streaming checkpoint scores during agent evaluation runs

### Data Pipeline

DuckDB for local analytics storage, Parquet for columnar I/O, InfluxDB for time-series streaming. GGUF converter handles MLX LoRA to GGUF tensor name mapping for model format conversion.

## go-inference Gap

This is the critical finding driving Phase 1.

**go-ml has**: `ml.Backend` interface where `Generate` returns `(string, error)`. Callback-based streaming via `StreamingBackend`.

**go-inference has**: `TextModel` interface where `Generate` returns `iter.Seq[Token]`. Iterator-based streaming (Go 1.23+ range-over-func).

**Gap**: No adapter between the two. `backend_mlx.go` imports go-mlx directly (~253 LOC of manual tokenisation, KV cache, sampling) instead of using go-inference which wraps all of that. This means:
1. MLX backend duplicates logic that go-inference already provides
2. Other backends (HTTP, Llama) cannot benefit from go-inference's unified interface
3. Scoring engine is locked to the legacy string-return interface

**Solution**: Write `InferenceAdapter` bridging `go-inference.TextModel` to `ml.Backend`, then rewrite `backend_mlx.go` to use go-inference. This is Phase 1 in TODO.md.

## Known Issues

- **backend_mlx.go imports go-mlx directly** — Should go through go-inference. ~253 LOC that collapses to ~60 LOC after migration.
- **agent.go is too large** — 1,070 LOC handling SSH, InfluxDB, scoring orchestration, and result publishing. Decomposition candidate.
- **Hardcoded infrastructure** — InfluxDB endpoint (`10.69.69.165:8181`), M3 SSH details baked into agent.go. Should be configurable.
- **No tests for backend_llama and backend_mlx** — Only backend_http_test.go exists for backends.
- **score.go concurrency untested** — Semaphore-bounded worker pool has no race condition tests.
