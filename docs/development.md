# go-ml Development Guide

## Prerequisites

### Required

- **Go 1.25** or later (the module uses `go 1.25.5`)
- **Go workspace** — go-ml is part of the `host-uk/core` Go workspace; `replace` directives in `go.mod` resolve sibling modules from local paths

### Required sibling modules (local paths)

| Module | Local path | Notes |
|--------|-----------|-------|
| `dappco.re/go/core` | `../go` | Framework, process management, logging |
| `forge.lthn.ai/core/go-inference` | `../go-inference` | Shared TextModel/Token interfaces |
| `dappco.re/go/core/mlx` | `../go-mlx` | Metal GPU backend |

All three must be checked out as siblings of `go-ml` (i.e. all four directories share the same parent).

### Platform-specific

- **Metal GPU (`NewMLXBackend`)** — requires macOS on Apple Silicon (darwin/arm64). The `backend_mlx.go` file carries a `//go:build darwin && arm64` build tag and is excluded on other platforms. All other features work on Linux and amd64.
- **llama-server** — the `llama-server` binary from llama.cpp must be on `PATH` or the path provided in `LlamaOpts.LlamaPath`.
- **DuckDB** — uses CGo; a C compiler (`gcc` or `clang`) is required.

---

## Getting Started

```bash
# On first checkout, populate go.sum
go mod download

# Verify the build (all platforms)
go build ./...

# Verify the build excluding Metal backend (Linux / CI)
GOFLAGS='-tags nomlx' go build ./...
```

---

## Build and Test Commands

```bash
# Run all tests
go test ./...

# Run with race detector (recommended before committing)
go test -race ./...

# Run a single test by name
go test -v -run TestHeuristic ./...
go test -v -run TestEngine_ScoreAll_ConcurrentSemantic ./...

# Run benchmarks
go test -bench=. ./...
go test -bench=BenchmarkHeuristicScore ./...

# Static analysis
go vet ./...

# Tidy dependencies
go mod tidy
```

---

## Test Patterns

### Naming convention

Tests use a `_Good`, `_Bad`, `_Ugly` suffix pattern:

- `_Good` — happy path (expected success)
- `_Bad` — expected error conditions (invalid input, unreachable server)
- `_Ugly` — panic and edge-case paths

### Mock backends

For tests that exercise `Backend`-dependent code (judge, agent, scoring engine) without a real inference server, implement `Backend` directly:

```go
type mockBackend struct {
    response string
    err      error
}

func (m *mockBackend) Generate(_ context.Context, _ string, _ ml.GenOpts) (string, error) {
    return m.response, m.err
}
func (m *mockBackend) Chat(_ context.Context, _ []ml.Message, _ ml.GenOpts) (string, error) {
    return m.response, m.err
}
func (m *mockBackend) Name() string    { return "mock" }
func (m *mockBackend) Available() bool { return true }
```

### Mock TextModel

For tests that exercise `InferenceAdapter` without Metal GPU hardware, implement `inference.TextModel`:

```go
type mockTextModel struct {
    tokens []string
    err    error
}

func (m *mockTextModel) Generate(ctx context.Context, prompt string, opts ...inference.GenerateOption) iter.Seq[inference.Token] {
    return func(yield func(inference.Token) bool) {
        for _, t := range m.tokens {
            if !yield(inference.Token{Text: t}) {
                return
            }
        }
    }
}
// ... implement remaining TextModel methods
func (m *mockTextModel) Err() error { return m.err }
```

### Mock RemoteTransport

For agent tests that would otherwise require an SSH connection:

```go
type fakeTransport struct {
    outputs map[string]string
    errors  map[string]error
}

func (f *fakeTransport) Run(_ context.Context, cmd string) (string, error) {
    if err, ok := f.errors[cmd]; ok {
        return "", err
    }
    return f.outputs[cmd], nil
}
func (f *fakeTransport) CopyFrom(_ context.Context, _, _ string) error { return nil }
func (f *fakeTransport) CopyTo(_ context.Context, _, _ string) error   { return nil }
```

Inject via `AgentConfig.Transport`:

```go
cfg := &ml.AgentConfig{
    Transport: &fakeTransport{outputs: map[string]string{...}},
}
```

### HTTP mock server

For `HTTPBackend` tests, use `net/http/httptest`:

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]any{
        "choices": []map[string]any{
            {"message": map[string]string{"role": "assistant", "content": "hello"}},
        },
    })
}))
defer srv.Close()
backend := ml.NewHTTPBackend(srv.URL, "test-model")
```

---

## Adding a New Backend

A backend must implement `ml.Backend`:

```go
type Backend interface {
    Generate(ctx context.Context, prompt string, opts GenOpts) (string, error)
    Chat(ctx context.Context, messages []Message, opts GenOpts) (string, error)
    Name() string
    Available() bool
}
```

### Steps

1. Create `backend_{name}.go` in the package root.
2. Add the `// SPDX-Licence-Identifier: EUPL-1.2` header.
3. Add a compile-time interface check:
   ```go
   var _ Backend = (*MyBackend)(nil)
   ```
4. Implement `Generate` as a thin wrapper around `Chat` where possible (follows the pattern of `HTTPBackend`).
5. Create `backend_{name}_test.go` with `_Good`, `_Bad`, and interface-compliance tests.
6. Register the backend in `service.go`'s `OnStartup` if it warrants lifecycle management, or document that callers must register it via `Service.RegisterBackend`.

### GPU backends

If the backend wraps a `go-inference.TextModel` (e.g. a new hardware accelerator), use `InferenceAdapter` rather than re-implementing the polling/streaming logic:

```go
m, err := myBackendPackage.LoadModel(modelPath)
if err != nil {
    return nil, err
}
return ml.NewInferenceAdapter(m, "my-backend"), nil
```

---

## Adding a New Scoring Suite

1. Add a new scoring function or type in a dedicated file (e.g. `my_suite.go`).
2. Add the suite name to `Engine.NewEngine`'s suite selection logic in `score.go`.
3. Add a result field to `PromptScore` in `types.go`.
4. Add the goroutine fan-out case in `Engine.ScoreAll` in `score.go`.
5. Add race condition tests in `score_race_test.go`.

---

## Coding Standards

### Language

Use **UK English** throughout: colour, organisation, centre, licence (noun), authorise. The only exception is identifiers in external APIs that use American spellings — do not rename those.

### File headers

Every new file must begin with:

```go
// SPDX-Licence-Identifier: EUPL-1.2
```

### Strict types

All parameters and return types must be explicitly typed. Avoid `interface{}` or `any` except at JSON unmarshalling boundaries.

### Import grouping

Three groups, each separated by a blank line:

```go
import (
    "context"           // stdlib
    "fmt"

    "dappco.re/go/core/log"               // dappco.re modules

    "forge.lthn.ai/core/go-inference"      // forge.lthn.ai (not yet migrated)

    "github.com/stretchr/testify/assert"   // third-party
)
```

### Error wrapping

Use `fmt.Errorf("context: %w", err)` for wrapping. Use `log.E("pkg.Type.Method", "what failed", err)` from the Core framework for structured error logging with stack context.

### Concurrency

- Protect shared maps with `sync.RWMutex` or `sync.Mutex` as appropriate.
- Use semaphore channels (buffered `chan struct{}`) to bound goroutine concurrency rather than `sync.Pool` or `errgroup` with fixed limits.
- Always check `model.Err()` after exhausting a `go-inference` token iterator — the iterator itself carries no error; the error is stored on the model.

---

## Conventional Commits

Use the following scopes:

| Scope | When to use |
|-------|-------------|
| `backend` | Changes to any `backend_*.go` file or the `adapter.go` bridge |
| `scoring` | Changes to `score.go`, `heuristic.go`, `judge.go`, `exact.go` |
| `probes` | Changes to `probes.go` or capability probe definitions |
| `agent` | Changes to any `agent_*.go` file |
| `service` | Changes to `service.go` or `Options` |
| `types` | Changes to `types.go` or `inference.go` interfaces |
| `gguf` | Changes to `gguf.go` |

Examples:

```
feat(backend): add ROCm backend via go-rocm InferenceAdapter
fix(scoring): handle nil ContentScores when content probe not found
refactor(agent): replace SSHCommand with SSHTransport.Run
test(probes): add Check function coverage for all 23 probes
```

---

## Co-Author and Licence

Every commit must include:

```
Co-Authored-By: Virgil <virgil@lethean.io>
```

The licence is **EUPL-1.2**. All source files carry the SPDX identifier in the header. Do not add licence headers to test files; the package-level declaration covers them.

---

## Forge Remote

The authoritative remote is `dappco.re/go/core/ml`:

```bash
git push forge main
```

The SSH remote URL is `ssh://git@forge.lthn.ai:2223/core/go-ml.git`. HTTPS authentication is not configured — always push via SSH.
