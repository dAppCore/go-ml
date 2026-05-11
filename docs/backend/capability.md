<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# capability.go — backend capability report

**Package**: `dappco.re/go/ml`
**File**: `go/capability.go`

## What this is

`CapabilityReportForBackend(name, backend)` — produces an `inference.CapabilityReport` for an `ml.Backend`. The bridge that lets go-ml's various backend types (HTTP, llama, MLX, Ollama, InferenceAdapter) all report through the **same shared report shape** that `core/api` and `core/ide` consume.

## CapabilityReportForBackend

```go
report := ml.CapabilityReportForBackend("mlx", backend)
```

Inspects the backend:

1. If it implements `inference.CapabilityReporter` (the InferenceAdapter path) → use the underlying model's report.
2. Otherwise → build a minimal report with `Backend: name`, `Available: backend.Available()`, and a fixed set of capabilities (`generate`, `chat`).

## Why this lives in go-ml

`inference.CapabilityReport` is owned by go-inference (portable shape). But go-ml has its own `Backend` interface that's slightly thinner than `inference.Backend` — building a report from a thinner type needs adapter-style logic, and that adapter logic belongs in go-ml.

## Used by

- `api/routes.go` — `/v1/ml/backends/{name}/capabilities` endpoint
- `Engine` introspection — score reports embed capability info
- `core/ide` (when wired through go-ml's API) — backend picker UI

## Related

- [inference.md](inference.md) — `ml.Backend` interface
- [adapter.md](adapter.md) — `InferenceAdapter` (where the rich path comes from)
- `../../../go-inference/docs/inference/capability.md` — CapabilityReport shape
