<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# backend_mlx.go — MLX convenience wrapper

**Package**: `dappco.re/go/ml`
**File**: `go/backend_mlx.go`
**Build tag**: `darwin && arm64 && !nomlx`

## What this is

The convenience entry that loads go-mlx and returns an `ml.Backend`. Wraps `mlx.NewMLXBackend(path)` (which itself returns a `*mlx.InferenceAdapter`) into an `ml.Backend` via the standard adapter chain.

A thin file — the real work happens in go-mlx and in `InferenceAdapter`. This file just gives ml-side callers a one-liner:

```go
backend, err := ml.NewMLXBackend("/models/gemma-4-e2b")
```

## Build tag

`//go:build darwin && arm64 && !nomlx` — exists only on Apple Silicon Macs without the `nomlx` build constraint. The stub variant compiles elsewhere and returns "not available" errors.

## API

```go
backend, err := ml.NewMLXBackend(modelPath, loadOpts ...inference.LoadOption)
```

Equivalent to:

```go
r := inference.LoadModel(modelPath, append(loadOpts, inference.WithBackend("metal"))...)
adapter := ml.NewInferenceAdapter(r.Value.(inference.TextModel), "mlx")
```

— but the convenience saves the type-assert dance.

## Why a separate file vs just using NewInferenceAdapter

Two reasons:

1. **Discoverability.** `ml.NewMLXBackend` is what someone reading scoring engine code expects to see. Forces them to learn about adapters anyway, but the entry name signals the intent.
2. **Build-tag isolation.** Stub variant compiles on other platforms; consumers can write `_ = ml.NewMLXBackend(...)` without #ifdef-style branches.

## Status

Production for darwin/arm64. Companion `_stub.go` returns errors elsewhere.

## Related

- [adapter.md](adapter.md) — `InferenceAdapter` that this wraps
- `../../../go-mlx/docs/runtime/adapter.md` — go-mlx's own InferenceAdapter (different layer)
- `../../../go-mlx/docs/runtime/register_metal.md` — what LoadModel uses
