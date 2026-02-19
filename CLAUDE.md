# CLAUDE.md

## What This Is

ML inference backends, scoring engine, and agent orchestrator. Module: `forge.lthn.ai/core/go-ml`

Provides pluggable inference backends (MLX/Metal, llama.cpp, HTTP/Ollama), a multi-suite scoring engine with ethics-aware probes, GGUF model management, and a concurrent worker pipeline for batch evaluation.

## Commands

```bash
go test ./...                    # Run all tests
go test -v -run TestHeuristic    # Single test
go test -bench=. ./...           # Benchmarks
```

## Architecture

### Backends (pluggable inference)

| File | Backend | Notes |
|------|---------|-------|
| `backend_mlx.go` | MLX/Metal GPU | Native Apple Silicon via go-mlx (darwin/arm64 only) |
| `backend_llama.go` | llama.cpp | GGUF models via subprocess |
| `backend_http.go` | HTTP API | Generic (Ollama, vLLM, OpenAI-compatible) |
| `ollama.go` | Ollama helpers | Ollama-specific client utilities |

### Scoring Engine

| File | Purpose |
|------|---------|
| `score.go` | Main scoring orchestrator |
| `heuristic.go` | Fast rule-based scoring (no LLM needed) |
| `judge.go` | LLM-as-judge evaluator |
| `exact.go` | Exact match scoring (GSM8K-style) |
| `probes.go` | Ethics-aware evaluation probes |

### Data Pipeline

| File | Purpose |
|------|---------|
| `agent.go` (1,070 LOC) | LLM agent orchestrator (largest file) |
| `worker.go` | Concurrent worker pool for multi-model scoring |
| `ingest.go` | Bulk data ingestion |
| `import_all.go` | Import orchestration |
| `gguf.go` | GGUF model handling and inventory |
| `convert.go` | Model format conversion |
| `db.go` | DuckDB storage layer |
| `parquet.go` | Parquet I/O |

### Monitoring

| File | Purpose |
|------|---------|
| `metrics.go` | Metrics tracking |
| `influx.go` | InfluxDB integration |
| `status.go` | Status reporting |

## Dependencies

- `forge.lthn.ai/core/go` — Framework (ServiceRuntime, process, log)
- `forge.lthn.ai/core/go-mlx` — Native Metal GPU inference
- `github.com/marcboeker/go-duckdb` — Embedded analytics DB
- `github.com/parquet-go/parquet-go` — Columnar data format

## Key Interfaces

```go
// Backend — pluggable inference
type Backend interface {
    Generate(ctx context.Context, prompt string, opts GenOpts) (string, error)
    Chat(ctx context.Context, messages []Message, opts GenOpts) (string, error)
    Name() string
    Available() bool
}

// StreamingBackend — extends Backend with token streaming
type StreamingBackend interface {
    Backend
    GenerateStream(ctx context.Context, prompt string, opts GenOpts, cb TokenCallback) error
    ChatStream(ctx context.Context, messages []Message, opts GenOpts, cb TokenCallback) error
}
```

## Coding Standards

- UK English
- Tests: testify assert/require
- Conventional commits
- Co-Author: `Co-Authored-By: Virgil <virgil@lethean.io>`
- Licence: EUPL-1.2

## Task Queue

See `TODO.md` for prioritised work.
See `FINDINGS.md` for research notes.
