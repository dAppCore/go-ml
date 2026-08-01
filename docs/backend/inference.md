<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# inference.go — ml.Backend interface

**Package**: `dappco.re/go/ml`
**File**: `go/inference.go`

## What this is

The **go-ml-side Backend interface** — the shape go-ml's scoring engine, agent loop, and CLI consume. Distinct from `inference.Backend` in `go-inference` (which is the GPU-backend-factory interface). go-ml chose a thinner Backend shape — Generate + Chat + Name + Available — that doesn't bind to iter.Seq tokens; backends that want GPU-level streaming are wrapped via `InferenceAdapter` ([adapter.md](adapter.md)) on the way in.

## Backend

```go
type Backend interface {
    Generate(ctx, prompt, GenOpts) core.Result   // value: ml.Result
    Chat(ctx, []Message, GenOpts) core.Result    // value: ml.Result
    Name() string
    Available() bool
}
```

`Result` is a buffered-string shape:

```go
type Result struct {
    Text    string
    Content string `json:"-"`                       // alt buffer (kept off-wire)
    Metrics *inference.GenerateMetrics             // populated by backends that can
}
```

## Why two Backend interfaces

The `inference.TextModel` (in go-inference) returns `iter.Seq[Token]` — token-level streaming. The `ml.Backend` returns a buffered string. Both have uses:

- **`inference.TextModel`** for hot-path inference and training (token-level control)
- **`ml.Backend`** for scoring and agent loops (the unit of consumption is a whole response, not tokens)

The adapter map (described in CLAUDE.md):

```
inference.TextModel ──► InferenceAdapter ──► ml.Backend       (adapter.go)
ml.HTTPBackend     ──► HTTPTextModel    ──► inference.TextModel  (backend_http_textmodel.go)
```

## GenOpts

```go
type GenOpts struct {
    Temperature float64
    MaxTokens   int
    Model       string      // override per-request
    TopK        int
    TopP        float64
    Stop        []string    // stop sequences (string-mode)
    // …
}

ml.DefaultGenOpts()  // sensible defaults
```

## Message

```go
type Message struct {
    Role    string  // "system" | "user" | "assistant"
    Content string
}
```

Same shape as `inference.Message` but defined locally to avoid forcing the inference import on consumers who only need ml-side types.

## Implementations

| Type | File | Purpose |
|------|------|---------|
| `InferenceAdapter` | `adapter.go` | Wraps any `inference.TextModel` |
| `HTTPBackend` | `backend_http.go` | OpenAI-compatible HTTP endpoint |
| `LlamaBackend` | `backend_llama.go` | Managed llama-server subprocess |
| MLX wrapper | `backend_mlx.go` | go-mlx convenience entry |
| Ollama | `ollama.go` | Ollama-native API |

## Used by

- `Engine.ScoreAll()` ([score.md](../scoring/score.md))
- `Judge` ([judge.md](../scoring/judge.md))
- `Agent` ([agent.md](../agent/agent.md))
- `api/routes.go` REST API

## Related

- [adapter.md](adapter.md) — `InferenceAdapter` bridges go-inference → ml.Backend
- [backend_http.md](backend_http.md) — HTTP backend
- [backend_llama.md](backend_llama.md) — llama-server subprocess
- [backend_mlx.md](backend_mlx.md) — MLX convenience
- `../../../go-inference/docs/inference/inference.md` — sibling-but-distinct interface
