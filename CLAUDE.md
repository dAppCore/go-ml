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

**This is the #1 priority.** Phase 1 in TODO.md.

The package currently defines its own `Backend` interface that returns `(string, error)`. The shared `go-inference` package defines `TextModel` which returns `iter.Seq[Token]` (Go 1.23+ range-over-func). Everything downstream is blocked until go-ml bridges these two interfaces.

### Interface Gap

```
go-ml (CURRENT)                         go-inference (TARGET)
─────────────────                        ─────────────────────
Backend.Generate(ctx, prompt, GenOpts)   TextModel.Generate(ctx, prompt, ...GenerateOption)
  → (string, error)                        → iter.Seq[Token]

Backend.Chat(ctx, messages, GenOpts)     TextModel.Chat(ctx, messages, ...GenerateOption)
  → (string, error)                        → iter.Seq[Token]

StreamingBackend.GenerateStream(         (streaming is built-in via iter.Seq)
  ctx, prompt, opts, TokenCallback)
  → error

GenOpts{Temperature, MaxTokens, Model}   GenerateConfig{MaxTokens, Temperature,
                                           TopK, TopP, StopTokens, RepeatPenalty}
                                         (configured via WithMaxTokens(n) etc.)
```

### What the Adapter Must Do

```go
// InferenceAdapter wraps go-inference.TextModel to satisfy ml.Backend + ml.StreamingBackend.
// This is the bridge between the new iterator-based API and the legacy string-return API.
type InferenceAdapter struct {
    model inference.TextModel
}

// Generate collects all tokens from the iterator into a string.
func (a *InferenceAdapter) Generate(ctx context.Context, prompt string, opts GenOpts) (string, error) {
    genOpts := convertOpts(opts) // GenOpts → []inference.GenerateOption
    var buf strings.Builder
    for tok := range a.model.Generate(ctx, prompt, genOpts...) {
        buf.WriteString(tok.Text)
    }
    if err := a.model.Err(); err != nil {
        return buf.String(), err
    }
    return buf.String(), nil
}

// GenerateStream yields tokens to the callback as they arrive.
func (a *InferenceAdapter) GenerateStream(ctx context.Context, prompt string, opts GenOpts, cb TokenCallback) error {
    genOpts := convertOpts(opts)
    for tok := range a.model.Generate(ctx, prompt, genOpts...) {
        if err := cb(tok.Text); err != nil {
            return err
        }
    }
    return a.model.Err()
}
```

### backend_mlx.go Is Broken

After go-mlx Phase 4, the old subpackage imports no longer exist:
- `forge.lthn.ai/core/go-mlx/cache` — **REMOVED** (now `internal/metal`)
- `forge.lthn.ai/core/go-mlx/model` — **REMOVED** (now `internal/metal`)
- `forge.lthn.ai/core/go-mlx/sample` — **REMOVED** (now `internal/metal`)
- `forge.lthn.ai/core/go-mlx/tokenizer` — **REMOVED** (now `internal/metal`)

The new go-mlx public API is:
```go
import (
    "forge.lthn.ai/core/go-inference"
    _ "forge.lthn.ai/core/go-mlx"  // registers "metal" backend via init()
)

m, err := inference.LoadModel("/path/to/model/", inference.WithContextLen(4096))
defer m.Close()
for tok := range m.Generate(ctx, "prompt", inference.WithMaxTokens(128)) {
    fmt.Print(tok.Text)
}
```

**The rewrite**: Delete the 253 LOC of manual tokenisation/KV cache/sampling. Replace with ~60 LOC that loads via go-inference and wraps in `InferenceAdapter`.

## Commands

```bash
go mod download                  # FIRST RUN: populate go.sum
go test ./...                    # Run all tests (some will fail until Phase 1)
go test -v -run TestHeuristic    # Single test
go test -bench=. ./...           # Benchmarks (none exist yet)
go test -race ./...              # Race detector
go vet ./...                     # Static analysis
```

**Note**: `backend_mlx.go` won't compile until rewritten (Phase 1) — it imports dead go-mlx subpackages. On darwin, the compiler will hit these broken imports. The Phase 1 rewrite fixes this by replacing the 253 LOC with ~60 LOC using go-inference.

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
| `backend_mlx.go` | MLX/Metal GPU | **BROKEN** — old imports, needs Phase 1 rewrite |
| `backend_llama.go` | llama-server subprocess | Works, needs go-inference wrapper |
| `backend_http.go` | HTTP API (OpenAI-compatible) | Works, needs go-inference wrapper |
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

### Key Types

```go
// Current backend interface (inference.go)
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

type GenOpts struct {
    Temperature float64
    MaxTokens   int
    Model       string
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}
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
