---
title: Backends
description: Pluggable inference backend interface and implementations (HTTP, llama.cpp, MLX).
---

# Backends

go-ml provides a `Backend` interface with four implementations. Backends can be used directly or registered with the `Service` for lifecycle management.

## Backend Interface

**File**: `inference.go`

```go
type Backend interface {
    Generate(ctx context.Context, prompt string, opts GenOpts) (Result, error)
    Chat(ctx context.Context, messages []Message, opts GenOpts) (Result, error)
    Name() string
    Available() bool
}
```

`Result` holds the response text and optional inference metrics (populated by backends that support them, such as MLX):

```go
type Result struct {
    Text    string
    Metrics *inference.GenerateMetrics
}
```

`GenOpts` configures each generation request:

```go
type GenOpts struct {
    Temperature   float64
    MaxTokens     int
    Model         string  // override model for this request
    TopK          int
    TopP          float64
    RepeatPenalty float64
}
```

`DefaultGenOpts()` returns `GenOpts{Temperature: 0.1}`.

`Message` is a type alias for `inference.Message`, providing backward compatibility:

```go
type Message = inference.Message
// Usage: ml.Message{Role: "user", Content: "Hello"}
```

### StreamingBackend (Deprecated)

For token-by-token streaming, `StreamingBackend` extends `Backend` with callback-based methods. New code should use `inference.TextModel` with `iter.Seq[Token]` directly via the `InferenceAdapter`.

```go
type StreamingBackend interface {
    Backend
    GenerateStream(ctx context.Context, prompt string, opts GenOpts, cb TokenCallback) error
    ChatStream(ctx context.Context, messages []Message, opts GenOpts, cb TokenCallback) error
}
```

## HTTPBackend

**File**: `backend_http.go`

Talks to any OpenAI-compatible `/v1/chat/completions` endpoint (Ollama, vLLM, llama-server, OpenAI).

```go
backend := ml.NewHTTPBackend("http://localhost:11434", "qwen3:8b")
backend.SetMaxTokens(2048)

result, err := backend.Generate(ctx, "Explain LoRA", ml.DefaultGenOpts())
```

**Features**:
- Automatic retry with exponential backoff (3 attempts) on transient failures (5xx, network errors)
- 300-second HTTP timeout
- Model override per request via `GenOpts.Model`
- Extracts response text from the first choice in the chat completions response

**Methods**: `Name() -> "http"`, `Available() -> true` (if baseURL is non-empty), `Model()`, `BaseURL()`.

### HTTPTextModel

**File**: `backend_http_textmodel.go`

Wraps `HTTPBackend` to satisfy the `inference.TextModel` interface, enabling HTTP backends to be used anywhere that expects a go-inference TextModel:

```go
textModel := ml.NewHTTPTextModel(backend)
for tok := range textModel.Generate(ctx, "Hello", inference.WithTemperature(0.5)) {
    fmt.Print(tok.Text)
}
```

Since the OpenAI-compatible API returns complete responses, `Generate` and `Chat` yield the entire response as a single token. `BatchGenerate` processes prompts sequentially.

## LlamaBackend

**File**: `backend_llama.go`

Manages a `llama-server` subprocess and delegates HTTP calls to it.

```go
backend := ml.NewLlamaBackend(processSvc, ml.LlamaOpts{
    LlamaPath: "/usr/local/bin/llama-server",
    ModelPath: "/models/gemma-3-1b.gguf",
    LoraPath:  "/adapters/lora.gguf",  // optional
    Port:      18090,
})

err := backend.Start(ctx)  // launches process, waits for health check (up to 30s)
defer backend.Stop()

result, err := backend.Generate(ctx, "Hello", ml.DefaultGenOpts())
```

**Lifecycle**:
1. `Start()` launches the llama-server process via `go-process` with the configured model and LoRA adapter
2. Polls `/health` every 500ms until the server responds 200 OK (30-second timeout)
3. `Generate`/`Chat` delegate to an internal `HTTPBackend` pointing at `127.0.0.1:{port}`
4. `Stop()` terminates the subprocess via `processSvc.Kill()`

`Available()` returns true only when the process is running and the health check passes.

### LlamaTextModel

Wraps `LlamaBackend` as an `inference.TextModel`. Embeds `HTTPTextModel` for generation, overrides `Close()` to stop the managed process:

```go
textModel := ml.NewLlamaTextModel(backend)
defer textModel.Close()  // stops llama-server
```

## MLX Backend

**File**: `backend_mlx.go`
**Build constraint**: `darwin && arm64`

Loads a model via `go-inference`'s Metal backend for native Apple Silicon GPU inference:

```go
adapter, err := ml.NewMLXBackend("/models/gemma-3-1b-4bit",
    inference.WithContextLength(4096),
)
defer adapter.Close()

result, err := adapter.Generate(ctx, "Hello", ml.DefaultGenOpts())
```

The blank import of `go-mlx` registers the "metal" backend, so `inference.LoadModel()` automatically uses Metal on Apple Silicon. Returns an `InferenceAdapter` that satisfies both `Backend` and `StreamingBackend`.

## InferenceAdapter

**File**: `adapter.go`

The key bridge between `go-inference` (iterator-based) and `go-ml` (string/callback-based). Wraps any `inference.TextModel` to satisfy `Backend` and `StreamingBackend`:

```go
adapter := ml.NewInferenceAdapter(textModel, "mlx")

// Backend methods (collect all tokens into a string):
result, err := adapter.Generate(ctx, "Hello", ml.DefaultGenOpts())

// StreamingBackend methods (forward each token to callback):
err := adapter.GenerateStream(ctx, "Hello", opts, func(token string) error {
    fmt.Print(token)
    return nil
})

// Access the underlying TextModel:
model := adapter.Model()
info := model.Info()
```

**GenOpts to inference options mapping**: Temperature, MaxTokens, TopK, TopP, and RepeatPenalty are forwarded. `GenOpts.Model` is ignored (the model is already loaded).

`InspectAttention` delegates to the underlying model if it implements `inference.AttentionInspector`:

```go
snapshot, err := adapter.InspectAttention(ctx, "Hello")
```

## Backend Selection Guide

| Backend | Use case | GPU | Latency | Setup |
|---------|----------|-----|---------|-------|
| `HTTPBackend` | Remote servers, Ollama, vLLM, OpenAI | Any | Network-bound | URL + model name |
| `LlamaBackend` | Local GGUF models via llama.cpp | CPU/GPU | Low | Binary + model path |
| `MLX (InferenceAdapter)` | Native Apple Silicon, Metal GPU | Apple M-series | Lowest | Model path |

## Worker Pipeline

**File**: `worker.go`

The LEM worker is an independent polling loop that fetches tasks from the LEM API, generates responses via an inference backend, and submits results:

```go
ml.RunWorkerLoop(&ml.WorkerConfig{
    APIBase:      "https://eaas.lthn.sh",
    WorkerID:     ml.MachineID(),
    Name:         ml.Hostname(),
    APIKey:       ml.ReadKeyFile(),
    GPUType:      "apple-m3",
    VRAMGb:       36,
    InferURL:     "http://localhost:11434",
    TaskType:     "generate",
    BatchSize:    5,
    PollInterval: 30 * time.Second,
})
```

The worker loop:
1. Registers with the LEM API (worker ID, GPU capabilities, supported models)
2. Polls `/api/lem/tasks/next` for available tasks
3. Claims each task, runs inference via OpenAI-compatible chat completions
4. Submits the response with generation time
5. Sends periodic heartbeats
6. Supports one-shot mode (`OneShot: true`) and dry-run mode
