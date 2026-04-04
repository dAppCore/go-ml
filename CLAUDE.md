# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Package Does

ML inference backends, scoring engine, and agent orchestrator for the Lethean AI stack (`dappco.re/go/core/ml`). Provides:

- **Pluggable inference backends** — MLX/Metal (darwin/arm64), llama.cpp (subprocess), HTTP/Ollama (OpenAI-compatible)
- **Multi-suite scoring engine** — heuristic (regex), semantic (LLM judge), content (sovereignty probes), standard benchmarks (TruthfulQA, DoNotAnswer, Toxigen, GSM8K), exact match
- **23 capability probes** — binary pass/fail tests across math, logic, reasoning, code, and word problem categories
- **GGUF model management** — format parsing, conversion, inventory
- **Agent orchestrator** — SSH checkpoint discovery, InfluxDB streaming, batch evaluation of fine-tuned adapters
- **CLI binary** — `cmd/lem/main.go` registers all subcommands via `cmd/cmd_*.go`
- **REST API** — `api/routes.go` exposes `/v1/ml/` endpoints via gin, implementing go-api's RouteGroup

See `docs/architecture.md` for the full architecture reference.

## Commands

```bash
go mod download                  # FIRST RUN: populate go.sum
go build ./...                   # Build all packages
go test ./...                    # Run all tests
go test -v -run TestHeuristic    # Single test
go test -bench=. ./...           # Benchmarks
go test -race ./...              # Race detector
go vet ./...                     # Static analysis

# Without native MLX library (Linux, CI, or macOS without libmlxc)
go build -tags nomlx ./...
go test -tags nomlx ./...
```

## Architecture

### Dual-interface backend system

Two interface families coexist, connected by adapters:

- **`ml.Backend`** (`inference.go`) — returns `Result` (text + optional metrics). Primary interface used by service, judge, agent, and scoring engine. Implementations: `HTTPBackend`, `LlamaBackend`, `InferenceAdapter`.
- **`inference.TextModel`** (in go-inference) — returns `iter.Seq[inference.Token]`. Natural API for GPU backends. Preferred for new code needing token-level control.

### Adapter map

```
inference.TextModel ──► InferenceAdapter ──► ml.Backend      (adapter.go)
ml.HTTPBackend     ──► HTTPTextModel    ──► inference.TextModel  (backend_http_textmodel.go)
ml.LlamaBackend    ──► LlamaTextModel   ──► inference.TextModel  (backend_http_textmodel.go)
```

`InferenceAdapter` bridges GPU backends into `ml.Backend`; the reverse adapters (`HTTPTextModel`, `LlamaTextModel`) allow HTTP/llama backends to be used where `inference.TextModel` is expected.

### Scoring engine (`score.go`)

`Engine.ScoreAll()` fans out across configured suites concurrently using a semaphore-bounded worker pool. Suites selected at construction via comma-separated string or `"all"`. Heuristic scoring runs inline (no LLM needed); semantic/content/standard use the LLM judge.

### Service layer (`service.go`)

`Service` embeds `core.ServiceRuntime[Options]` for Core framework lifecycle. `OnStartup` registers backends and initialises the judge/engine. Backends can also be registered at runtime via `RegisterBackend`.

### Agent orchestrator (`agent_*.go`)

SSH-based checkpoint discovery on remote M3 Mac, probe evaluation, InfluxDB/DuckDB result streaming. `RemoteTransport` interface abstracts SSH for testability.

## Dependencies

Sibling modules in the Core Go ecosystem (must be checked out as siblings for local development if `replace` directives are active):

| Module | Purpose |
|--------|---------|
| `dappco.re/go/core` | Framework (ServiceRuntime, process, log) |
| `forge.lthn.ai/core/go-mlx` | Metal GPU backend (darwin/arm64 only) — not yet migrated |
| `forge.lthn.ai/core/go-inference` | Shared TextModel/Backend interfaces — not yet migrated |
| `forge.lthn.ai/core/cli` | CLI framework — not yet migrated |

Platform-specific: `backend_mlx.go` has `//go:build darwin && arm64 && !nomlx`. Use `-tags nomlx` to exclude the Metal backend when `libmlxc` is not installed. DuckDB requires CGo (C compiler).

## Coding Standards

- **UK English**: colour, organisation, centre, licence (noun)
- **SPDX header**: `// SPDX-Licence-Identifier: EUPL-1.2` in every new source file
- **Tests**: testify assert/require; `_Good`/`_Bad`/`_Ugly` suffix pattern
- **Imports**: stdlib → dappco.re → forge.lthn.ai → third-party, each group separated by blank line. Within a group, sort alphabetically.
- **Concurrency**: semaphore channels (`chan struct{}`) for bounding goroutines; always check `model.Err()` after exhausting a token iterator
- **Licence**: EUPL-1.2

## Commit Conventions

Format: `type(scope): description`

**Scopes**: `backend`, `scoring`, `probes`, `agent`, `service`, `types`, `gguf`

```
Co-Authored-By: Virgil <virgil@lethean.io>
```

## Repository

- **Module**: `dappco.re/go/core/ml`
- **Push via SSH**: `git push forge main` (remote: `ssh://git@forge.lthn.ai:2223/core/go-ml.git`)
