---
title: go-ml
description: ML inference backends, scoring engine, and agent orchestrator for Go.
---

# go-ml

`dappco.re/go/core/ml` provides pluggable inference backends, a multi-suite scoring engine with ethics-aware probes, GGUF model management, and a concurrent worker pipeline for batch evaluation.

**Module**: `dappco.re/go/core/ml`
**Size**: ~7,500 LOC across 41 Go files, 6 test files
**Licence**: EUPL-1.2

## Core Capabilities

| Area | Description |
|------|-------------|
| **Inference backends** | MLX (Metal GPU), llama.cpp (subprocess), HTTP (Ollama/vLLM/OpenAI-compatible) |
| **Scoring engine** | Heuristic (regex), semantic (LLM judge), exact match, ethics probes |
| **Agent orchestrator** | Multi-model scoring runs with concurrent worker pool |
| **Model management** | GGUF format parsing, MLX-to-PEFT conversion, Ollama model creation |
| **Data pipeline** | DuckDB storage, Parquet I/O, InfluxDB metrics, HuggingFace publishing |

## Dependencies

| Module | Purpose |
|--------|---------|
| `core/go` | Framework services, lifecycle, process management |
| `core/go-inference` | Shared `TextModel`/`Backend`/`Token` interfaces |
| `core/go-mlx` | Native Metal GPU inference (darwin/arm64) |
| `core/go-process` | Subprocess management for llama-server |
| `core/go-log` | Structured error helpers |
| `core/go-api` | REST API route registration |
| `go-duckdb` | Embedded analytics database |
| `parquet-go` | Columnar data format |

## Architecture Overview

The package is organised around four layers:

```
Backend Layer     — inference.go, backend_http.go, backend_llama.go, backend_mlx.go
                    Pluggable backends behind a common Backend interface.

Scoring Layer     — score.go, heuristic.go, judge.go, exact.go, probes.go
                    Multi-suite concurrent scoring engine.

Agent Layer       — agent_execute.go, agent_eval.go, agent_influx.go, agent_ssh.go
                    Orchestrates checkpoint discovery, evaluation, and result publishing.

Data Layer        — db.go, influx.go, parquet.go, export.go, io.go
                    Storage, metrics, and training data pipeline.
```

## Service Registration

`go-ml` integrates with the Core DI framework via `Service`:

```go
import "dappco.re/go/core/ml"

c, _ := core.New(
    core.WithName("ml", ml.NewService(ml.Options{
        OllamaURL:  "http://localhost:11434",
        JudgeURL:   "http://localhost:11434",
        JudgeModel: "qwen3:8b",
        Suites:     "all",
        Concurrency: 4,
    })),
)
```

On startup, the service registers configured backends and initialises the scoring engine. It implements `Startable` and `Stoppable` for lifecycle integration.

### Service Methods

```go
svc.Generate(ctx, "ollama", "Explain LoRA", ml.DefaultGenOpts())
svc.ScoreResponses(ctx, responses)
svc.RegisterBackend("custom", myBackend)
svc.Backend("ollama")
svc.DefaultBackend()
svc.Judge()
svc.Engine()
```

## REST API

The `api` sub-package provides Gin-based REST endpoints. See [HTTP API](api.md) for the full endpoint reference, including the standalone `ml serve` server.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/ml/backends` | List registered backends with availability |
| `GET` | `/v1/ml/status` | Service readiness, backend list, judge availability |
| `POST` | `/v1/ml/generate` | Generate text against a named backend |

WebSocket channels: `ml.generate`, `ml.status`.

## Quick Start

```go
// Create an HTTP backend pointing at Ollama
backend := ml.NewHTTPBackend("http://localhost:11434", "qwen3:8b")

// Generate text
result, err := backend.Generate(ctx, "What is LoRA?", ml.DefaultGenOpts())
fmt.Println(result.Text)

// Score a batch of responses
judge := ml.NewJudge(backend)
engine := ml.NewEngine(judge, 4, "heuristic,semantic")
scores := engine.ScoreAll(ctx, responses)
```

## Further Reading

- [HTTP API](api.md) -- `ml serve` endpoints, `api/` RouteGroup contract, auth notes, curl examples
- [Scoring Engine](scoring.md) -- Heuristic analysis, LLM judge, probes, benchmarks
- [Backends](backends.md) -- HTTP, llama.cpp, MLX, and the inference adapter
- [Training Pipeline](training.md) -- Data export, LoRA conversion, adapter management
- [Model Management](models.md) -- GGUF parsing, Ollama integration, checkpoint discovery
