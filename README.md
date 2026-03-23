[![Go Reference](https://pkg.go.dev/badge/dappco.re/go/core/ml.svg)](https://pkg.go.dev/dappco.re/go/core/ml)
[![License: EUPL-1.2](https://img.shields.io/badge/License-EUPL--1.2-blue.svg)](LICENSE.md)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go)](go.mod)

# go-ml

ML inference backends, multi-suite scoring engine, and agent orchestrator for the Lethean AI stack. The package provides pluggable backends (Apple Metal via go-mlx, managed llama-server subprocesses, and OpenAI-compatible HTTP APIs), a concurrent scoring engine that evaluates model responses across heuristic, semantic, content, and standard benchmark suites, 23 capability probes, GGUF model management, and an SSH-based agent orchestrator that streams checkpoint evaluation results to InfluxDB and DuckDB.

**Module**: `dappco.re/go/core/ml`
**Licence**: EUPL-1.2
**Language**: Go 1.25

## Quick Start

```go
import "dappco.re/go/core/ml"

// HTTP backend (Ollama, LM Studio, any OpenAI-compatible endpoint)
backend := ml.NewHTTPBackend("http://localhost:11434", "qwen3:8b")
resp, err := backend.Generate(ctx, "Hello", ml.GenOpts{MaxTokens: 256})

// Scoring engine
engine := ml.NewEngine(backend, ml.Options{Suites: "heuristic,semantic", Concurrency: 4})
scores := engine.ScoreAll(responses)
```

## Documentation

- [Architecture](docs/architecture.md) — backends, scoring engine, agent orchestrator, data pipeline
- [Development Guide](docs/development.md) — building, testing, contributing
- [Project History](docs/history.md) — completed phases and known limitations

## Build & Test

```bash
go test ./...
go test -race ./...
go test -bench=. ./...
go build ./...
```

## Licence

European Union Public Licence 1.2 — see [LICENCE](LICENCE) for details.
