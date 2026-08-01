<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# backend_http.go — HTTPBackend (OpenAI-compatible)

**Package**: `dappco.re/go/ml`
**File**: `go/backend_http.go` (plus `backend_http_textmodel.go` for the reverse adapter)

## What this is

The **HTTP backend** — talks OpenAI-compatible `/v1/chat/completions` to any endpoint (OpenAI proper, Ollama, vLLM, llama.cpp server, Violet, the go-inference openai handler, etc.). Implements `ml.Backend`. The widest-compatibility entry into go-ml.

## Config

```go
type HTTPConfig struct {
    BaseURL    string             // e.g. "http://localhost:8080"
    APIKey     string             // optional, sent as Authorization Bearer
    Model      string             // default model id
    Timeout    time.Duration      // per-request
    HTTPClient *http.Client       // override for custom transport
}

backend, err := ml.NewHTTPBackend(cfg)
```

## Backend methods

```go
backend.Generate(ctx, prompt, opts) core.Result   // r.Value = ml.Result
backend.Chat(ctx, []Message, opts) core.Result
backend.Name() string                              // "http" or per-config
backend.Available() bool                           // ping-checks BaseURL
```

## Wire path

`Chat`:

1. Build `ChatCompletionRequest` (from `inference/openai` DTOs)
2. POST `{BaseURL}/v1/chat/completions` with JSON body
3. Decode `ChatCompletionResponse`
4. Extract `Choices[0].Message.Content` → `ml.Result.Text`

No streaming on this path; the backend buffers the response.

## HTTPTextModel (reverse adapter)

In `backend_http_textmodel.go`:

```go
m := ml.NewHTTPTextModel(backend, "model-id")    // returns inference.TextModel
```

Wraps an `HTTPBackend` to implement `inference.TextModel`. Use case: go-ml is using HTTP for scoring, but some piece of code wants `inference.TextModel` (e.g., to pass through `inference.LoadModel`'s shape). The wrapper bridges back.

## Why both directions

The two adapter directions ([adapter.md](adapter.md) and HTTPTextModel) let any combination of "native gpu + http" interplay through either interface. The scoring engine prefers `ml.Backend`; the provider router prefers `inference.TextModel`. Both flows are supported.

## Available

Default `Available()` does a lightweight HEAD or `/v1/models` probe. Cached for a short window (avoiding ping storms when the engine builds reports across many backends).

## Used by

- `Service.OnStartup` — register configured HTTP endpoints
- Any test fixture pointing at a local server
- Adapter to talk to Violet from go-ml's scoring loop

## Related

- [inference.md](inference.md) — Backend interface
- [adapter.md](adapter.md) — opposite-direction adapter
- [backend_llama.md](backend_llama.md) — sibling that manages a subprocess
- `../../../go-inference/docs/openai/openai.md` — wire DTOs shared
