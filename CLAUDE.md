# CLAUDE.md — go-ml Agent Guide

You are a dedicated domain expert for `forge.lthn.ai/core/go-ml`. Virgil (in core/go) orchestrates work. Pick up tasks in phase order, mark `[x]` when done, commit and push.

## What This Package Does

ML inference backends, scoring engine, and agent orchestrator. ~7,500 LOC across 41 Go files. Provides:

- **Pluggable inference backends** — MLX/Metal (darwin/arm64), llama.cpp (subprocess), HTTP/Ollama (OpenAI-compatible)
- **Multi-suite scoring engine** — heuristic (regex), semantic (LLM judge), content (sovereignty probes), standard benchmarks (TruthfulQA, DoNotAnswer, Toxigen, GSM8K)
- **23 capability probes** — binary pass/fail tests across 16 categories
- **GGUF model management** — format parsing, conversion, inventory
- **Agent orchestrator** — SSH checkpoint discovery, InfluxDB streaming, batch evaluation

See `docs/architecture.md` for the full architecture reference.

## Commands

```bash
go mod download                  # FIRST RUN: populate go.sum
go test ./...                    # Run all tests
go test -v -run TestHeuristic    # Single test
go test -bench=. ./...           # Benchmarks
go test -race ./...              # Race detector
go vet ./...                     # Static analysis
```

## Local Dependencies

All resolve via `replace` directives in go.mod:

| Module | Local Path | Notes |
|--------|-----------|-------|
| `forge.lthn.ai/core/go` | `../go` | Framework (ServiceRuntime, process, log) |
| `forge.lthn.ai/core/go-mlx` | `../go-mlx` | Metal GPU backend (darwin/arm64 only) |
| `forge.lthn.ai/core/go-inference` | `../go-inference` | Shared TextModel/Backend interfaces |

## Coding Standards

- **UK English**: colour, organisation, centre, licence (noun)
- **SPDX header**: `// SPDX-Licence-Identifier: EUPL-1.2` in every new source file
- **Tests**: testify assert/require; `_Good`/`_Bad`/`_Ugly` suffix pattern
- **Conventional commits**: `feat(backend):`, `fix(scoring):`, `refactor(agent):`
- **Co-Author**: `Co-Authored-By: Virgil <virgil@lethean.io>`
- **Licence**: EUPL-1.2
- **Imports**: stdlib → forge.lthn.ai → third-party, each group separated by blank line

## Forge

- **Repo**: `forge.lthn.ai/core/go-ml`
- **Push via SSH**: `git push forge main` (remote: `ssh://git@forge.lthn.ai:2223/core/go-ml.git`)
