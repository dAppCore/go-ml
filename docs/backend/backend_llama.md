<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# backend_llama.go — LlamaBackend (managed llama-server)

**Package**: `dappco.re/go/ml`
**File**: `go/backend_llama.go`

## What this is

The **managed llama-server backend** — go-ml starts and supervises a local `llama-server` (llama.cpp's HTTP server) subprocess, then talks to it via its OpenAI-compatible HTTP surface. Implements `ml.Backend`.

Two responsibilities:

1. Process lifecycle — spawn, healthcheck, restart on crash, shutdown clean
2. HTTP client — same as HTTPBackend, but pointed at the managed local port

## Config

```go
type LlamaConfig struct {
    BinaryPath    string             // path to llama-server binary
    ModelPath     string             // GGUF model path
    Port          int                // local listen port
    ContextSize   int                // -c flag
    NumGPULayers  int                // -ngl flag
    Threads       int                // -t flag
    ExtraArgs     []string           // pass-through CLI args
    StartTimeout  time.Duration      // wait for ready ping
    Env           map[string]string  // process env
}
```

## Backend methods

```go
backend.Start(ctx) error                              // spawn the process
backend.Stop(ctx) error                               // SIGTERM + wait
backend.Generate(ctx, prompt, opts) core.Result       // routes through HTTP
backend.Chat(ctx, []Message, opts) core.Result
backend.Name() string                                 // "llama"
backend.Available() bool                              // child running + ping ok
```

## Lifecycle

```
Start:
  spawn llama-server with --port, --model, --ngl, --ctx-size args
  ↓
  poll http://localhost:Port/health until ok or StartTimeout
  ↓
  store cmd handle; ready to receive Generate/Chat

Generate / Chat:
  delegate to internal HTTP client (same wire as HTTPBackend)

Stop:
  send SIGTERM
  wait for graceful exit
  if timeout, SIGKILL
```

## Crash recovery

`Available()` checks both child-alive and HTTP-healthcheck. If `Available() == false`, callers can `Restart()` to re-spawn — or the service's supervisor logic does so automatically when configured.

## Why a separate file from HTTPBackend

Two distinct concerns:

- **HTTPBackend** doesn't own a process — it dials an external endpoint.
- **LlamaBackend** owns a process — start, supervise, restart, stop.

The Backend interface is the same; the operational ownership is different.

## Why bundled with go-ml

go-ml is the scoring + eval lane. Eval often needs reproducible local inference without depending on an external service. A managed llama-server gives that — `go-ml eval` boots its own llama-server, runs the eval, shuts it down. No external dependencies for CI runs.

## Status

Production for CPU-only workloads and llama.cpp-supported GGUFs. Doesn't compete with go-mlx for darwin/arm64 (Metal is faster); does compete on linux without ROCm/CUDA.

## Used by

- `Service.OnStartup` — auto-start configured backends
- CI eval runs (the canonical "give me a reproducible local LLM" path)
- Tests that need a real model without GPU dependencies

## Related

- [inference.md](inference.md) — Backend interface
- [backend_http.md](backend_http.md) — sibling for non-managed HTTP endpoints
- [backend_mlx.md](backend_mlx.md) — sibling for native MLX
- llama.cpp project — the upstream binary
