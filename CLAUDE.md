# CLAUDE.md — go-ml Domain Expert Guide

You are a dedicated domain expert for `forge.lthn.ai/core/go-ml`. Virgil (in core/go) orchestrates your work via TODO.md. Pick up tasks in phase order, mark `[x]` when done, commit and push.

## What This Package Does

ML inference backends, scoring engine, and agent orchestrator. 7.5K LOC across 41 Go files. Provides:

- **Pluggable inference backends** — MLX/Metal (darwin/arm64), llama.cpp (subprocess), HTTP/Ollama (OpenAI-compatible)
- **Multi-suite scoring engine** — Heuristic (regex), semantic (LLM judge), content (sovereignty probes), standard benchmarks (TruthfulQA, DoNotAnswer, Toxigen, GSM8K)
- **23 capability probes** — Binary pass/fail tests across 16 categories (math, logic, code, etc.)
- **GGUF model management** — Format parsing, conversion, inventory
- **Agent orchestrator** — SSH checkpoint discovery, InfluxDB streaming, batch evaluation

## Critical Context: go-inference Migration

**Phase 1 is complete.** Both directions of the bridge are implemented:

1. **Forward adapter** (`adapter.go`): `inference.TextModel` (iter.Seq) -> `ml.Backend`/`ml.StreamingBackend` (string/callback). Used by `backend_mlx.go` to wrap Metal GPU models.
2. **Reverse adapters** (`backend_http_textmodel.go`): `HTTPBackend`/`LlamaBackend` -> `inference.TextModel`. Enables HTTP and llama-server backends to be used anywhere that expects a go-inference TextModel.

### Interface Bridge (DONE)

```
ml.Backend (string)  <──adapter.go──>  inference.TextModel (iter.Seq[Token])
                     <──backend_http_textmodel.go──>
```

- `InferenceAdapter`: TextModel -> Backend + StreamingBackend (for MLX, ROCm, etc.)
- `HTTPTextModel`: HTTPBackend -> TextModel (for remote APIs)
- `LlamaTextModel`: LlamaBackend -> TextModel (for managed llama-server)

### backend_mlx.go (DONE)

Rewritten from 253 LOC to ~35 LOC. Loads via `inference.LoadModel()` and wraps in `InferenceAdapter`. Uses go-mlx's Metal backend registered via `init()`.

### Downstream Consumers Verified

- `service.go` — `Service.Generate()` calls `Backend.Generate()`. InferenceAdapter satisfies Backend. No changes needed.
- `judge.go` — `Judge.judgeChat()` calls `Backend.Generate()`. Same contract, works as before.

## Commands

```bash
go mod download                  # FIRST RUN: populate go.sum
go test ./...                    # Run all tests
go test -v -run TestHeuristic    # Single test
go test -bench=. ./...           # Benchmarks (none exist yet)
go test -race ./...              # Race detector
go vet ./...                     # Static analysis
```

## Local Dependencies

All resolve via `replace` directives in go.mod:

| Module | Local Path | Notes |
|--------|-----------|-------|
| `forge.lthn.ai/core/go` | `../host-uk/core` | Framework (ServiceRuntime, process, log) |
| `forge.lthn.ai/core/go-mlx` | `../go-mlx` | Metal GPU backend (darwin/arm64 only) |
| `forge.lthn.ai/core/go-inference` | `../go-inference` | Shared TextModel/Backend interfaces |

## Architecture

### Backends (pluggable inference)

| File | Backend | Status |
|------|---------|--------|
| `adapter.go` | InferenceAdapter (TextModel -> Backend) | DONE — bridges go-inference to ml.Backend |
| `backend_mlx.go` | MLX/Metal GPU | DONE — uses go-inference LoadModel + InferenceAdapter |
| `backend_http.go` | HTTP API (OpenAI-compatible) | Works as ml.Backend |
| `backend_http_textmodel.go` | HTTPTextModel + LlamaTextModel | DONE — reverse wrappers (Backend -> TextModel) |
| `backend_llama.go` | llama-server subprocess | Works as ml.Backend |
| `ollama.go` | Ollama helpers | Works |

### Scoring Engine

| File | LOC | Purpose |
|------|-----|---------|
| `score.go` | 212 | Concurrent scoring orchestrator (semaphore-bounded workers) |
| `heuristic.go` | 258 | 9 regex-based metrics, LEK composite score |
| `judge.go` | 205 | LLM-as-judge (6 scoring methods) |
| `exact.go` | 77 | GSM8K exact-match with numeric extraction |
| `probes.go` | 273 | 23 binary capability probes across 16 categories |

### Data Pipeline

| File | LOC | Purpose |
|------|-----|---------|
| `agent.go` | 1,070 | Scoring agent (SSH checkpoint discovery, InfluxDB) |
| `worker.go` | 403 | LEM API worker for distributed inference |
| `service.go` | 162 | Core framework integration (lifecycle, backend registry) |
| `ingest.go` | 384 | JSONL response loading |
| `db.go` | 258 | DuckDB analytics storage |
| `gguf.go` | 369 | GGUF model format parsing |

### Backend Architecture

Two interface families coexist, bridged by adapters:

**`inference.TextModel`** (iterator-based) is the **preferred API** for new code. Returns `iter.Seq[inference.Token]` for streaming. Defined in `forge.lthn.ai/core/go-inference`. Use this for GPU backends (MLX Metal, ROCm) and any code that needs token-level control.

**`ml.Backend`** (string-based) is the **compatibility layer**, still fully supported. Returns complete strings. Used by `service.go`, `judge.go`, and external consumers like `host-uk/cli`.

**`ml.StreamingBackend`** is **deprecated**. New code should use `inference.TextModel` with `iter.Seq[Token]` directly. Retained for backward compatibility with existing callers.

**Adapters:**

| Adapter | Direction | File |
|---------|-----------|------|
| `InferenceAdapter` | `inference.TextModel` -> `ml.Backend` + `ml.StreamingBackend` | `adapter.go` |
| `HTTPTextModel` | `ml.HTTPBackend` -> `inference.TextModel` | `backend_http_textmodel.go` |
| `LlamaTextModel` | `ml.LlamaBackend` -> `inference.TextModel` | `backend_http_textmodel.go` |

**Unified types (Phase 2):**

- `ml.Message` is a type alias for `inference.Message` — the types are identical, no conversion needed between packages.
- `ml.GenOpts` extends `inference.GenerateConfig` with a `Model` field for per-request model overrides. The `convertOpts()` helper maps GenOpts to `[]inference.GenerateOption`.

### Key Types

```go
// Backend interface (inference.go) — compatibility layer
type Backend interface {
    Generate(ctx context.Context, prompt string, opts GenOpts) (string, error)
    Chat(ctx context.Context, messages []Message, opts GenOpts) (string, error)
    Name() string
    Available() bool
}

// Deprecated: use inference.TextModel with iter.Seq[Token] directly
type StreamingBackend interface {
    Backend
    GenerateStream(ctx context.Context, prompt string, opts GenOpts, cb TokenCallback) error
    ChatStream(ctx context.Context, messages []Message, opts GenOpts, cb TokenCallback) error
}

type GenOpts struct {
    Temperature   float64
    MaxTokens     int
    Model         string  // override model for this request
    TopK          int     // top-k sampling (0 = disabled)
    TopP          float64 // nucleus sampling threshold (0 = disabled)
    RepeatPenalty float64 // repetition penalty (0 = disabled, 1.0 = no penalty)
}

// Type alias — identical to inference.Message
type Message = inference.Message
```

## Coding Standards

- **UK English**: colour, organisation, centre
- **Tests**: testify assert/require (existing), Pest-style names welcome for new tests
- **Conventional commits**: `feat(backend):`, `fix(scoring):`, `refactor(mlx):`
- **Co-Author**: `Co-Authored-By: Virgil <virgil@lethean.io>`
- **Licence**: EUPL-1.2
- **Imports**: stdlib → forge.lthn.ai → third-party, each group separated by blank line

## Forge

- **Repo**: `forge.lthn.ai/core/go-ml`
- **Push via SSH**: `git push forge main` (remote: `ssh://git@forge.lthn.ai:2223/core/go-ml.git`)

## Task Queue

See `TODO.md` for prioritised work. Phase 1 (go-inference migration) is the critical path.
See `FINDINGS.md` for research notes and interface mapping.
