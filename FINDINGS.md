# FINDINGS.md — go-ml Research & Discovery

## 2026-02-19: Split from go-ai (Virgil)

### Origin

Extracted from `forge.lthn.ai/core/go-ai/ml/`. Zero internal go-ai dependencies — imports go-mlx (external module) and core/go framework only.

### What Was Extracted

- 41 Go files (~7,494 LOC excluding tests)
- 6 test files (backend_http, exact, heuristic, judge, probes, score)

### Key Finding: Heaviest Package

ml/ is 53% of go-ai's total LOC. After extraction, go-ai drops from ~14K to ~3.4K LOC (ai/ facade + mcp/ hub).

### Dependencies

- `forge.lthn.ai/core/go-mlx` — Metal GPU inference (backend_mlx.go, darwin/arm64 only)
- `forge.lthn.ai/core/go` — Framework services, process management, logging
- `github.com/marcboeker/go-duckdb` — Analytics storage
- `github.com/parquet-go/parquet-go` — Columnar data I/O

### Consumers

- `go-ai/mcp/tools_ml.go` — Exposes ML as MCP tools
- `go-ai/test-mlx.go` — Integration test utility
- LEM Lab — Uses MLXBackend for chat inference

### Architecture Note: agent.go

At 1,070 LOC, agent.go is the largest file. It orchestrates:
- Multi-model scoring runs
- Remote M3 infrastructure scheduling
- Ethics-aware probe evaluation
- Result consolidation and publishing

This file is a decomposition candidate but functional as-is.
