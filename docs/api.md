# go-ml HTTP API

## Overview

`go-ml` exposes two HTTP surfaces:

1. **Standalone `ml serve` server** -- an OpenAI-compatible `net/http` server started by the CLI. It serves completions, chat completions, model discovery, model metadata, health, and the bundled chat UI.
2. **`api/` RouteGroup** -- a Gin route group for hosts using `dappco.re/go/api`. It mounts service-level ML endpoints under `/v1/ml`.

The two surfaces are separate. `ml serve` builds its own `http.ServeMux`; the `api/` package implements the `go-api` `RouteGroup` and `StreamGroup` contracts.

Source anchors:

| Surface | Code |
|---------|------|
| `ml serve` mux creation and route registration | [`cmd/cmd_serve.go:67-86`](../cmd/cmd_serve.go#L67-L86) |
| `ml serve` UI routes | [`cmd/cmd_serve.go:88-101`](../cmd/cmd_serve.go#L88-L101) |
| model list and model-info registration | [`cmd/cmd_serve.go:148-168`](../cmd/cmd_serve.go#L148-L168) |
| model-info response shape | [`cmd/cmd_serve.go:185-223`](../cmd/cmd_serve.go#L185-L223) |
| completion and chat handlers | [`cmd/cmd_serve.go:240-580`](../cmd/cmd_serve.go#L240-L580) |
| `api/` route group contract | [`api/routes.go:15-38`](../api/routes.go#L15-L38) |
| `api/` stream channels | [`api/routes.go:40-43`](../api/routes.go#L40-L43) |
| `api/` response handlers | [`api/routes.go:71-137`](../api/routes.go#L71-L137) |

---

## `ml serve`

`ml serve` defaults to `0.0.0.0:8090` and serves the single loaded backend. It is OpenAI-compatible for `/v1/completions`, `/v1/chat/completions`, and `/v1/models`.

### Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/healthz` | Runtime health for the standalone server |
| `GET` | `/v1/models` | OpenAI-compatible model list |
| `GET` | `/v1/models/{id}/info` | CoreGO model metadata for the loaded model |
| `POST` | `/v1/completions` | OpenAI-compatible text completion |
| `POST` | `/v1/chat/completions` | OpenAI-compatible chat completion |
| `GET` | `/` | Bundled chat UI |
| `GET` | `/chat.js` | Bundled chat UI JavaScript |

There is no `/v1/health`, `/v1/embeddings`, or `/v1/embed` endpoint in the current `ml serve` implementation.

### `GET /healthz`

No request body.

Response:

```json
{
  "status": "ok",
  "model": "gemma3-4b-it-q4",
  "uptime_seconds": 42,
  "active_requests": 0,
  "max_threads": 8,
  "max_tokens": 4096,
  "max_context": 4
}
```

Error codes:

| Status | Meaning |
|--------|---------|
| `200` | Server is running |
| `405` | Method does not match the registered route |

Example:

```bash
curl http://localhost:8090/healthz
```

### `GET /v1/models`

No request body.

Response:

```json
{
  "object": "list",
  "data": [
    {
      "id": "gemma3-4b-it-q4"
    }
  ]
}
```

Error codes:

| Status | Meaning |
|--------|---------|
| `200` | Model list returned |
| `405` | Method does not match the registered route |

Example:

```bash
curl http://localhost:8090/v1/models
```

### `GET /v1/models/{id}/info`

`id` must match the loaded backend name. URL-encoded model IDs are accepted by the Go router before comparison.

No request body.

Response:

```json
{
  "id": "gemma3-4b-it-q4",
  "architecture": "gemma3",
  "vocab_size": 256000,
  "num_layers": 34,
  "hidden_size": 3072,
  "quant_bits": 4,
  "quant_group": 32,
  "prefill_tokens_per_sec": 712.4,
  "decode_tokens_per_sec": 82.6,
  "peak_memory_bytes": 8589934592,
  "active_memory_bytes": 6442450944
}
```

Only `id` is guaranteed. The remaining fields are omitted when the backend does not expose an `inference.TextModel` with model info and metrics.

Error codes:

| Status | Meaning |
|--------|---------|
| `200` | Metadata returned for the loaded model |
| `404` | `{id}` does not match the loaded backend name |
| `405` | Method does not match the registered route |

Example:

```bash
curl http://localhost:8090/v1/models/gemma3-4b-it-q4/info
```

### `POST /v1/completions`

Request body:

```json
{
  "model": "gemma3-4b-it-q4",
  "prompt": "Explain LoRA in one paragraph.",
  "max_tokens": 256,
  "temperature": 0.7,
  "stream": false
}
```

`max_tokens` is capped to the server's `--max-tokens` value. If `max_tokens` is `0`, the server cap is used. `model` is passed into `ml.GenOpts` for backends that support per-request model selection; the standalone server still serves one loaded backend.

Non-streaming response:

```json
{
  "id": "cmpl-1770000000000000000",
  "object": "text_completion",
  "created": 1770000000,
  "model": "gemma3-4b-it-q4",
  "choices": [
    {
      "text": "LoRA is ...",
      "index": 0,
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 0,
    "completion_tokens": 0,
    "total_tokens": 0
  }
}
```

When `stream` is `true` and the backend implements `ml.StreamingBackend`, the response is `text/event-stream`:

```text
data: {"id":"cmpl-1770000000000000000","object":"text_completion","created":1770000000,"model":"gemma3-4b-it-q4","choices":[{"text":"Lo","index":0,"finish_reason":null}]}

data: {"id":"cmpl-1770000000000000000","object":"text_completion","created":1770000000,"model":"gemma3-4b-it-q4","choices":[{"text":"","index":0,"finish_reason":"stop"}]}

data: [DONE]
```

If `stream` is `true` but the backend is not a `StreamingBackend`, the handler falls back to the non-streaming response.

Error codes:

| Status | Meaning |
|--------|---------|
| `200` | Completion returned, or stream opened |
| `400` | Request body is not valid JSON |
| `429` | `--max-requests` concurrency limit reached |
| `500` | Backend generation failed, or streaming flush support is unavailable |
| `405` | Method does not match the registered route |

Example:

```bash
curl http://localhost:8090/v1/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gemma3-4b-it-q4",
    "prompt": "Explain LoRA in one paragraph.",
    "max_tokens": 256,
    "temperature": 0.7
  }'
```

### `POST /v1/chat/completions`

Request body:

```json
{
  "model": "gemma3-4b-it-q4",
  "messages": [
    {
      "role": "system",
      "content": "You are concise."
    },
    {
      "role": "user",
      "content": "What is LoRA?"
    }
  ],
  "max_tokens": 256,
  "temperature": 0.7,
  "stream": false
}
```

Messages use the OpenAI-style `{ "role": "...", "content": "..." }` shape. `role` is normally `system`, `user`, or `assistant`. If the request has more messages than `--max-context`, the server preserves the first system message when present and keeps the last `--max-context` non-system messages.

Non-streaming response:

```json
{
  "id": "chatcmpl-1770000000000000000",
  "object": "chat.completion",
  "created": 1770000000,
  "model": "gemma3-4b-it-q4",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "LoRA is ..."
      },
      "index": 0,
      "finish_reason": "stop"
    }
  ]
}
```

When `stream` is `true` and the backend implements `ml.StreamingBackend`, the response is `text/event-stream`. The first chunk declares the assistant role, following chunks carry `delta.content`, and the stream ends with `[DONE]`:

```text
data: {"id":"chatcmpl-1770000000000000000","object":"chat.completion.chunk","created":1770000000,"model":"gemma3-4b-it-q4","choices":[{"delta":{"role":"assistant"},"index":0,"finish_reason":null}]}

data: {"id":"chatcmpl-1770000000000000000","object":"chat.completion.chunk","created":1770000000,"model":"gemma3-4b-it-q4","choices":[{"delta":{"content":"LoRA"},"index":0,"finish_reason":null}]}

data: {"id":"chatcmpl-1770000000000000000","object":"chat.completion.chunk","created":1770000000,"model":"gemma3-4b-it-q4","choices":[{"delta":{},"index":0,"finish_reason":"stop"}]}

data: [DONE]
```

If `stream` is `true` but the backend is not a `StreamingBackend`, the handler falls back to the non-streaming response.

Error codes:

| Status | Meaning |
|--------|---------|
| `200` | Chat completion returned, or stream opened |
| `400` | Request body is not valid JSON |
| `429` | `--max-requests` concurrency limit reached |
| `500` | Backend chat generation failed, or streaming flush support is unavailable |
| `405` | Method does not match the registered route |

Examples:

```bash
curl http://localhost:8090/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gemma3-4b-it-q4",
    "messages": [
      {"role": "user", "content": "What is LoRA?"}
    ],
    "max_tokens": 256,
    "temperature": 0.7
  }'
```

```bash
curl -N http://localhost:8090/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "messages": [
      {"role": "user", "content": "Stream a short answer."}
    ],
    "stream": true
  }'
```

### UI helper routes

`GET /` returns the bundled chat HTML. `GET /chat.js` returns its JavaScript. These routes are same-origin helpers for local interactive use; they do not add a separate API contract.

---

## `api/` RouteGroup

The `dappco.re/go/ml/api` package exposes an embeddable Gin route group around `*ml.Service`.

Contract:

| Method | Value |
|--------|-------|
| `NewRoutes(svc)` | Constructs `*Routes` with the provided service |
| `Name()` | `ml` |
| `BasePath()` | `/v1/ml` |
| `RegisterRoutes(rg)` | Mounts `GET /backends`, `GET /status`, `POST /generate` |
| `Channels()` | `ml.generate`, `ml.status` |

Registration:

```go
var svc *ml.Service // resolved from the host Core service registry

engine, _ := api.New()
engine.Register(mlapi.NewRoutes(svc))
handler := engine.Handler()
```

`go-api` mounts each group at `BasePath()`, so the relative handlers become `/v1/ml/backends`, `/v1/ml/status`, and `/v1/ml/generate`.

The route group does not register `/v1/ml/health`. Hosts that use `go-api` get the framework's root `GET /health` endpoint separately from the ML routes.

All `api/` responses use the standard `go-api` envelope:

```json
{
  "success": true,
  "data": {}
}
```

Failures use:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "human-readable message"
  }
}
```

Middleware configured on the host `go-api` engine may add additional statuses such as `401`, `403`, `429`, or `504`.

### `GET /v1/ml/backends`

No request body.

Response:

```json
{
  "success": true,
  "data": [
    {
      "name": "ollama",
      "available": true
    }
  ]
}
```

Error codes:

| Status | Code | Meaning |
|--------|------|---------|
| `200` | n/a | Backend list returned |
| `503` | `SERVICE_UNAVAILABLE` | Route group was created with a nil service |
| `404` | n/a | Path not registered, including method/path combinations not mounted by Gin |

Example:

```bash
curl http://localhost:8080/v1/ml/backends
```

### `GET /v1/ml/status`

No request body.

Response:

```json
{
  "success": true,
  "data": {
    "ready": true,
    "backends": ["ollama"],
    "has_judge": true
  }
}
```

`ready` is true when at least one backend is registered. `has_judge` is true when the service has an initialised judge.

Error codes:

| Status | Code | Meaning |
|--------|------|---------|
| `200` | n/a | Status returned |
| `503` | `SERVICE_UNAVAILABLE` | Route group was created with a nil service |
| `404` | n/a | Path not registered, including method/path combinations not mounted by Gin |

Example:

```bash
curl http://localhost:8080/v1/ml/status
```

### `POST /v1/ml/generate`

Request body:

```json
{
  "prompt": "Explain LoRA in one paragraph.",
  "backend": "ollama",
  "temperature": 0.7,
  "max_tokens": 256
}
```

`prompt` is required by Gin binding. `backend` is optional; if it is empty or unknown, `ml.Service.Generate` falls back to the configured default backend. Positive `temperature` and `max_tokens` override `ml.DefaultGenOpts()`.

Response:

```json
{
  "success": true,
  "data": {
    "text": "LoRA is ..."
  }
}
```

Error codes:

| Status | Code | Meaning |
|--------|------|---------|
| `200` | n/a | Text generated |
| `400` | `INVALID_REQUEST` | JSON binding failed, including a missing `prompt` when the service is initialised |
| `500` | `GENERATION_FAILED` | The service failed to generate text |
| `503` | `SERVICE_UNAVAILABLE` | Route group was created with a nil service |
| `404` | n/a | Path not registered, including method/path combinations not mounted by Gin |

Example:

```bash
curl http://localhost:8080/v1/ml/generate \
  -H 'Content-Type: application/json' \
  -d '{
    "prompt": "Explain LoRA in one paragraph.",
    "backend": "ollama",
    "max_tokens": 256,
    "temperature": 0.7
  }'
```

---

## Authentication And Scoping

`ml serve` does not implement authentication, authorisation, tenant scoping, or API keys. The default bind address is `0.0.0.0:8090`, so deployments that expose it beyond a trusted host should put it behind a reverse proxy or firewall that supplies the required controls.

`api/` does not add route-level authentication inside `api/routes.go`. Authentication, request scoping, CORS, rate limiting, response metadata, and other policies are inherited from the surrounding `go-api` engine and middleware configuration. The only ML-specific scope is the injected `*ml.Service`: `/v1/ml/backends` only lists backends registered on that service, and `/v1/ml/generate` only uses that service's default or named backends.
