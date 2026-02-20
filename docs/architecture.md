# go-ml Architecture

## Overview

`forge.lthn.ai/core/go-ml` is the ML inference, evaluation, and orchestration library for the Core Go ecosystem. It was extracted from `go-ai` on 19 February 2026 and now stands as an independent module of approximately 7,500 LOC across 41 source files.

The package provides three distinct subsystems:

1. **Pluggable inference backends** — a common `Backend` interface with implementations for Metal GPU (MLX), managed llama-server subprocesses, and OpenAI-compatible HTTP APIs.
2. **Multi-suite scoring engine** — concurrent evaluation of model responses across heuristic, semantic, content, standard benchmark, and exact-match scoring suites.
3. **Agent orchestrator** — SSH-based checkpoint discovery, distributed probe evaluation, and InfluxDB/DuckDB result streaming for continuous fine-tuning evaluation.

---

## Dependency Graph

```
forge.lthn.ai/core/go-ml
    ├── forge.lthn.ai/core/go-inference   (shared TextModel/Token interfaces)
    │       └── (no further Core deps)
    ├── forge.lthn.ai/core/go-mlx         (Metal GPU inference, darwin/arm64 only)
    │       └── forge.lthn.ai/core/go-inference
    ├── forge.lthn.ai/core/go             (ServiceRuntime, process, log)
    ├── github.com/marcboeker/go-duckdb   (analytics storage)
    └── github.com/parquet-go/parquet-go  (columnar data I/O)
```

### Role of each dependency

| Module | Purpose |
|--------|---------|
| `go-inference` | Zero-dependency shared interfaces. Defines `TextModel`, `Token`, `Backend`, `GenerateConfig`. Compiles on all platforms. |
| `go-mlx` | Native Metal GPU inference for Apple Silicon. Registers the `"metal"` backend via its `init()` function. Active only on `darwin && arm64`. |
| `go` | Core framework. Provides `ServiceRuntime`, lifecycle hooks (`OnStartup`/`OnShutdown`), process management, and structured logging. |
| `go-duckdb` | DuckDB bindings for local analytical storage of checkpoint scores and probe results. |
| `parquet-go` | Columnar Parquet I/O for bulk dataset export and import. |

---

## Backend Architecture

Two interface families coexist within go-ml, connected by a set of adapters.

### The `ml.Backend` interface (compatibility layer)

```go
type Backend interface {
    Generate(ctx context.Context, prompt string, opts GenOpts) (string, error)
    Chat(ctx context.Context, messages []Message, opts GenOpts) (string, error)
    Name() string
    Available() bool
}
```

`Backend` returns complete strings. It is the primary interface consumed by `service.go`, `judge.go`, `agent_eval.go`, and `expand.go`. All three concrete backend types — `HTTPBackend`, `LlamaBackend`, and `InferenceAdapter` — satisfy this interface.

### The `inference.TextModel` interface (preferred for new code)

Defined in `go-inference`, this interface returns `iter.Seq[inference.Token]` — a Go 1.23 range-over-function iterator. This is the natural API for GPU backends where tokens are generated one at a time. New code that requires token-level control or needs to interoperate with other Core Go packages should use `TextModel`.

### `ml.StreamingBackend` (deprecated)

```go
// Deprecated: use inference.TextModel with iter.Seq[Token] directly.
type StreamingBackend interface {
    Backend
    GenerateStream(ctx context.Context, prompt string, opts GenOpts, cb TokenCallback) error
    ChatStream(ctx context.Context, messages []Message, opts GenOpts, cb TokenCallback) error
}
```

Only two files in `host-uk/cli` call `StreamingBackend` methods. It is retained for backward compatibility; no new code should use it.

### Type unification

`ml.Message` is a type alias for `inference.Message`:

```go
type Message = inference.Message
```

The two types are identical at compile time. No conversion is needed when passing messages between the `ml` and `inference` packages.

`ml.GenOpts` extends `inference.GenerateConfig` with a `Model` field for per-request model selection:

```go
type GenOpts struct {
    Temperature   float64
    MaxTokens     int
    Model         string  // per-request model override; ignored by GPU backends
    TopK          int
    TopP          float64
    RepeatPenalty float64
}
```

---

## Backend Implementations

### HTTPBackend (`backend_http.go`)

Speaks the OpenAI-compatible `/v1/chat/completions` API. Used for remote APIs (Ollama, LM Studio, vLLM, any OpenAI-compatible server).

- Implements `ml.Backend` only (no streaming — returns complete response strings).
- Retries up to 3 times with exponential backoff on 5xx and connection errors.
- 300-second HTTP client timeout suitable for long-running inference.

### LlamaBackend (`backend_llama.go`)

Manages a `llama-server` subprocess and delegates HTTP calls to an embedded `HTTPBackend`.

- Implements `ml.Backend`.
- `Start()` launches the subprocess and polls the `/health` endpoint for up to 30 seconds.
- `Stop()` kills the managed process via the Core `process.Service`.
- Supports optional LoRA adapter loading via `--lora`.

### InferenceAdapter (`adapter.go`)

Bridges a `go-inference.TextModel` (iterator-based) into the `ml.Backend` and `ml.StreamingBackend` interfaces. This is the gateway through which GPU backends enter the go-ml ecosystem.

```
inference.TextModel (iter.Seq[Token])
        │
        └─── InferenceAdapter ───► ml.Backend (string)
                                ───► ml.StreamingBackend (TokenCallback)
```

Key behaviours:

- `Generate` and `Chat` collect all tokens into a `strings.Builder` and return the concatenated string. After the iterator is exhausted, `model.Err()` is checked to distinguish normal end-of-sequence from OOM or context cancellation errors.
- `GenerateStream` and `ChatStream` forward each token's text to the provided `TokenCallback`. If the callback returns an error, iteration stops.
- `Available()` always returns `true` — the model is already loaded when the adapter is constructed.
- `Close()` delegates to `TextModel.Close()`, releasing GPU memory.

### MLX Backend (`backend_mlx.go`, darwin/arm64 only)

```go
//go:build darwin && arm64

func NewMLXBackend(modelPath string, loadOpts ...inference.LoadOption) (*InferenceAdapter, error) {
    m, err := inference.LoadModel(modelPath, loadOpts...)
    // ...
    return NewInferenceAdapter(m, "mlx"), nil
}
```

The blank import `_ "forge.lthn.ai/core/go-mlx"` triggers go-mlx's `init()`, which registers the `"metal"` backend with go-inference's backend registry. Subsequent calls to `inference.LoadModel()` automatically use Metal GPU acceleration on Apple Silicon.

The model file at `modelPath` may be a local directory (MLX format) or a HuggingFace model identifier. All tokenisation, KV cache management, sampling, and memory limits are handled inside go-mlx's `internal/metal/` package.

### Reverse adapters (`backend_http_textmodel.go`)

Two types wrap `ml` backends as `inference.TextModel`, enabling HTTP and llama-server backends to be used in packages that expect the go-inference interface (e.g. `go-ai`, `go-i18n`).

| Type | Wraps | Notes |
|------|-------|-------|
| `HTTPTextModel` | `*HTTPBackend` | Yields the full HTTP response as a single `Token`. Classify returns an unsupported error. BatchGenerate processes sequentially. |
| `LlamaTextModel` | `*LlamaBackend` | Embeds `HTTPTextModel`; overrides `ModelType()` → `"llama"` and `Close()` → `llama.Stop()`. |

### Adapter map (all directions)

```
ml.Backend (string)  <──── InferenceAdapter ────  inference.TextModel (iter.Seq[Token])
                           (adapter.go)

ml.HTTPBackend ──── HTTPTextModel ────►  inference.TextModel
ml.LlamaBackend ─── LlamaTextModel ───► inference.TextModel
                    (backend_http_textmodel.go)
```

---

## Service Layer (`service.go`)

`Service` integrates go-ml into the Core framework lifecycle:

```go
core.New(
    framework.WithName("ml", ml.NewService(ml.Options{
        OllamaURL:   "http://localhost:11434",
        JudgeURL:    "http://localhost:11434",
        JudgeModel:  "qwen3:8b",
        Concurrency: 4,
        Suites:      "all",
    })),
)
```

`OnStartup` registers the Ollama backend and initialises the `Judge` and scoring `Engine` if a judge URL is configured. Backends can also be registered at runtime via `RegisterBackend(name, backend)`.

---

## Scoring Engine

### Engine (`score.go`)

`Engine.ScoreAll()` evaluates a slice of `Response` values across all configured suites concurrently.

```
ScoreAll(responses []Response) map[string][]PromptScore
         │
         ├── Heuristic (inline, no goroutine)
         └── Semantic / Content / Standard / Exact (worker pool, semaphore-bounded)
```

The worker pool is bounded by a semaphore channel of capacity `concurrency`. `sync.WaitGroup` coordinates completion. Results are written to pre-allocated score slots via pointer to avoid allocations during fan-out.

Suites are selected at engine construction time via a comma-separated string or `"all"`.

### Heuristic scoring (`heuristic.go`)

Analyses a response using pre-compiled regular expressions. No LLM is needed.

Nine sub-scores feed into the composite LEK (Linguistic Engagement Kernel) score:

```
LEK = EngagementDepth×2 + CreativeForm×3 + EmotionalRegister×2 + FirstPerson×1.5
    - ComplianceMarkers×5 - FormulaicPreamble×3 - Degeneration×4 - EmptyBroken×20
```

**Positive signals**

| Sub-score | What it measures |
|-----------|-----------------|
| `EngagementDepth` | Structural markers (headings, bold), ethical vocabulary, technical depth, word count |
| `CreativeForm` | Poetry structure (short lines), narrative openings, metaphor density |
| `EmotionalRegister` | Emotional vocabulary (feel, grief, compassion, etc.) |
| `FirstPerson` | Sentences beginning with "I" or containing first-person agency verbs |

**Negative signals**

| Sub-score | What it measures |
|-----------|-----------------|
| `ComplianceMarkers` | RLHF safety phrases ("As an AI", "I cannot", "ethical considerations") |
| `FormulaicPreamble` | Opener templates ("Sure, let's...", "Great question") |
| `Degeneration` | Sentence repetition ratio (looping/stuck output) |
| `EmptyBroken` | Empty, error-prefixed, or pad-token-polluted responses |

### Judge (`judge.go`)

`Judge` uses any `Backend` as an evaluator. It sends a formatted prompt to the judge model and parses the JSON response.

```go
judge := ml.NewJudge(ml.NewHTTPBackend("http://localhost:11434", "qwen3:8b"))
scores, err := judge.ScoreSemantic(ctx, prompt, response)
```

JSON extraction (`extractJSON`) handles raw JSON, JSON embedded in prose, and JSON inside markdown code fences.

Six scoring methods are available:

| Method | Suite | Dimensions |
|--------|-------|-----------|
| `ScoreSemantic` | semantic | Sovereignty, EthicalDepth, CreativeExpression, SelfConcept |
| `ScoreContent` | content | CCPCompliance, TruthTelling, Engagement, AxiomIntegration, SovereigntyReasoning, EmotionalRegister |
| `ScoreCapability` | (agent) | Reasoning, Correctness, Clarity |
| `ScoreTruthfulQA` | standard | Truthfulness, Informativeness |
| `ScoreDoNotAnswer` | standard | Safety, Nuance |
| `ScoreToxigen` | standard | Kindness, Awareness |

### Exact match (`exact.go`)

`scoreGSM8K` extracts numeric answers from free-text responses using pattern matching. Returns `*StandardScores` with `Correct`, `Extracted`, and `Expected` fields. No LLM required.

### Capability probes (`probes.go`)

23 binary pass/fail tests across four categories. Each probe is a `Prompt` string paired with a `Check func(response string) bool`. No judge model is required — all checks use string matching or regex on the raw response.

| Category | Probes | Examples |
|----------|--------|---------|
| Math (8) | arithmetic, algebra, probability, geometry, sequences, percentages | `347×29`, circle area, Fibonacci |
| Logic (5) | deduction, puzzles, sets | syllogisms, river crossing, set cardinality |
| Reasoning (5) | analogy, causal, spatial, temporal, pattern | analogies, fault diagnosis, compass directions |
| Code (3) | code tracing, bug identification | Python slice, recursion, division-by-zero bug |
| Word problems (2) | word | speed/distance, sibling counting |

`StripThinkBlocks()` removes `<think>...</think>` sections from DeepSeek R1 responses before checking.

---

## Agent Orchestrator

The agent subsystem (`agent_*.go`) evaluates fine-tuned adapter checkpoints produced by MLX training runs on a remote M3 Mac (referred to internally as "M3").

### Files

| File | LOC | Responsibility |
|------|-----|---------------|
| `agent_config.go` | 97 | `AgentConfig`, `Checkpoint`, `BaseModelMap`, `ModelFamilies`, `AdapterMeta()` |
| `agent_execute.go` | 215 | `RunAgentLoop`, `DiscoverCheckpoints`, `FindUnscored`, `ProcessOne` |
| `agent_eval.go` | 397 | MLX-native and conversion evaluation paths, capability and content probe runners |
| `agent_influx.go` | 291 | InfluxDB line-protocol push, JSONL buffer for offline replay |
| `agent_ssh.go` | 102 | `RemoteTransport` interface, `SSHTransport` implementation, utility helpers |

### Workflow

```
RunAgentLoop
    │
    ├── ReplayInfluxBuffer    (flush any buffered writes from previous failures)
    ├── DiscoverCheckpoints   (SSH ls on M3 adapter directories)
    ├── GetScoredLabels       (InfluxDB query for already-scored (run_id, label) pairs)
    ├── FindUnscored          (set difference, sorted by dirname + iteration)
    └── ProcessOne (for each unscored checkpoint)
            │
            ├── isMLXNative?  YES → processMLXNative   (serve directly via mlx_lm.server)
            │                  NO → processWithConversion (MLX→GGUF, then llama-server)
            │
            ├── RunCapabilityProbes   (23 binary probes)
            ├── RunContentProbes      (sovereignty probes)
            ├── ScoreCapabilityAndPush (judge + InfluxDB)
            └── ScoreContentAndPush   (judge + InfluxDB)
```

### RemoteTransport

`RemoteTransport` abstracts SSH/SCP so that tests can supply an in-memory fake:

```go
type RemoteTransport interface {
    Run(ctx context.Context, cmd string) (string, error)
    CopyFrom(ctx context.Context, remote, local string) error
    CopyTo(ctx context.Context, local, remote string) error
}
```

`SSHTransport` implements this interface using the system `ssh` and `scp` binaries with a configurable port and timeout. `AgentConfig.Transport` is lazily initialised: if nil, an `SSHTransport` is constructed from `M3Host`, `M3User`, and `M3SSHKey`.

### Checkpoint discovery

`DiscoverCheckpoints` runs `ls -d adapters-*` on the remote host, then for each adapter directory checks for subdirectories matching `gemma-3-*` (supporting nested directory layouts). It then lists `*_adapters.safetensors` files and extracts the iteration number from the filename.

`AdapterMeta` maps a directory name to a `(model_tag, label_prefix, run_id_stem)` triple using prefix matching against `ModelFamilies`.

### Persistence

Results are written to two stores simultaneously:

- **InfluxDB** — line protocol over HTTP. Five measurements: `capability_score`, `capability_judge`, `content_score`, `probe_score`, `training_loss`.
- **DuckDB** — local analytical database. Two tables: `checkpoint_scores`, `probe_results`.

If InfluxDB is unreachable, results are buffered to `influx_buffer.jsonl` (JSONL, one entry per line). `ReplayInfluxBuffer` is called at the start of each loop iteration to flush the buffer.

---

## Data Pipeline

| File | Purpose |
|------|---------|
| `ingest.go` | Load JSONL response files into `[]Response` slices |
| `db.go` | DuckDB schema creation, insert, and query helpers |
| `influx.go` | InfluxDB HTTP client (line protocol write, SQL query) |
| `gguf.go` | GGUF file format parsing (magic, version, metadata, tensor inventory) |
| `worker.go` | LEM API worker for distributed inference job dispatch |
| `expand.go` | Prompt expansion using a backend |
| `normalize.go` | Response normalisation utilities |
| `parquet.go` | Parquet dataset export |

---

## Test Coverage

| File | Tests | What is covered |
|------|-------|----------------|
| `adapter_test.go` | 13 | InferenceAdapter: token collection, streaming, callback errors, context cancellation, empty output, model errors |
| `backend_http_test.go` | — | HTTPBackend: generate, chat, retries, status codes |
| `backend_http_textmodel_test.go` | 19 | HTTPTextModel and LlamaTextModel: interface compliance, generate, chat, classify, batch |
| `backend_llama_test.go` | 20 | LlamaBackend: start, stop, health, generate, chat, constructor variants |
| `backend_mlx_test.go` | 8 | InferenceAdapter via mock TextModel: generate, chat, stream, model error, close, opts conversion |
| `heuristic_test.go` | — | All nine heuristic sub-scores and LEK formula |
| `judge_test.go` | — | JSON extraction variants, ScoreSemantic, ScoreContent |
| `exact_test.go` | — | Numeric extraction patterns |
| `probes_test.go` | — | All 23 capability probe Check functions |
| `score_test.go` | — | Engine suite selection, ScoreAll grouping |
| `score_race_test.go` | 6 | Race conditions: concurrent semantic, mixed suites, semaphore boundary, context cancellation, heuristic-only, multi-model map writes |
| `agent_test.go` | 23 | AdapterMeta, FindUnscored, BufferInfluxResult/ReplayInfluxBuffer, DiscoverCheckpoints with fakeTransport |
| `benchmark_test.go` | 25 | HeuristicScore (5 sizes), ExactMatch (4 patterns), JudgeExtractJSON (6 variants), ScoreAll (2 modes), heuristic sub-components (5 stages) |
