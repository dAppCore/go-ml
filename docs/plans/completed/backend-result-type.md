# Backend Result Type — Completion Summary

**Completed:** 22 February 2026
**Module:** `forge.lthn.ai/core/go-ml`
**Status:** Complete — unified Result struct across all backends

## What Was Built

Refactored Generate/Chat return types across all ML backends (HTTP, Llama,
MLX adapter) to use a unified `Result` struct carrying both generated text
and inference metrics.

### Result struct

```go
type Result struct {
    Text    string
    Metrics Metrics
}

type Metrics struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
    LatencyMs        float64
    TokensPerSecond  float64
}
```

### Backends updated

- HTTP backend (Ollama, OpenAI-compatible endpoints)
- Llama backend (llama.cpp via CGo)
- MLX adapter (delegates to go-mlx)

All tests updated to use the new return type. No breaking changes to the
public `ml.Generate()` / `ml.Chat()` API — the Result struct is returned
where previously only a string was.
