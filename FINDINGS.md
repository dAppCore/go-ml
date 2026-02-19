# FINDINGS.md — go-ml Research & Discovery

## 2026-02-19: Split from go-ai (Virgil)

### Origin

Split from go-ai on 19 Feb 2026. Was `ai/ml/` subpackage inside `forge.lthn.ai/core/go-ai`. Zero internal go-ai dependencies — imports go-mlx (external module) and core/go framework only.

### What Was Extracted

- 41 Go files (~7,494 LOC excluding tests)
- 6 test files (backend_http, exact, heuristic, judge, probes, score)
- ml/ was 53% of go-ai's total LOC. After extraction, go-ai drops from ~14K to ~3.4K LOC (ai/ facade + mcp/ hub).

### Dependencies

- `forge.lthn.ai/core/go-mlx` — Metal GPU inference (backend_mlx.go, darwin/arm64 only)
- `forge.lthn.ai/core/go-inference` — Shared TextModel/Backend/Token interfaces (target for Phase 1)
- `forge.lthn.ai/core/go` — Framework services, process management, logging
- `github.com/marcboeker/go-duckdb` — Analytics storage
- `github.com/parquet-go/parquet-go` — Columnar data I/O
- `github.com/stretchr/testify` — Test assertions

### Consumers

- `go-ai/mcp/tools_ml.go` — Exposes ML as MCP tools (uses `ml.Service`, `ml.GenOpts`, `ml.Backend`)
- LEM Lab — Uses MLXBackend for chat inference
- go-i18n Phase 2a — Needs 5K sentences/sec Gemma3-1B classification (blocked on go-inference)

## go-inference Interface Mapping

### Type Correspondence

| go-ml | go-inference | Notes |
|-------|-------------|-------|
| `ml.Backend` | `inference.Backend` | Different semantics: ml returns string, inference returns TextModel |
| `ml.StreamingBackend` | (built into TextModel) | iter.Seq[Token] is inherently streaming |
| `ml.GenOpts` | `inference.GenerateConfig` | Use functional options: `WithMaxTokens(n)` etc. |
| `ml.Message` | `inference.Message` | Identical struct: Role + Content |
| `ml.TokenCallback` | (not needed) | iter.Seq[Token] replaces callbacks |
| (no equivalent) | `inference.Token` | `{ID int32, Text string}` |
| (no equivalent) | `inference.TextModel` | Generate/Chat return iter.Seq[Token] |

### Method Mapping

```
ml.Backend.Generate(ctx, prompt, GenOpts) → (string, error)
   ↕ InferenceAdapter collects tokens
inference.TextModel.Generate(ctx, prompt, ...GenerateOption) → iter.Seq[Token]

ml.StreamingBackend.GenerateStream(ctx, prompt, opts, TokenCallback) → error
   ↕ InferenceAdapter forwards tokens to callback
inference.TextModel.Generate(ctx, prompt, ...GenerateOption) → iter.Seq[Token]

ml.GenOpts{Temperature: 0.7, MaxTokens: 2048}
   ↕ convertOpts helper
inference.WithTemperature(0.7), inference.WithMaxTokens(2048)
```

### backend_mlx.go Before/After

**Before** (253 LOC — BROKEN, old subpackage imports):
```go
import (
    "forge.lthn.ai/core/go-mlx"
    "forge.lthn.ai/core/go-mlx/cache"    // REMOVED
    "forge.lthn.ai/core/go-mlx/model"    // REMOVED
    "forge.lthn.ai/core/go-mlx/sample"   // REMOVED
    "forge.lthn.ai/core/go-mlx/tokenizer"// REMOVED
)

type MLXBackend struct {
    model      model.Model
    tok        *tokenizer.Tokenizer
    caches     []cache.Cache
    sampler    sample.Sampler
    // ... manual tokenisation, KV cache mgmt, sampling loop, memory cleanup
}
```

**After** (~60 LOC — uses go-inference + InferenceAdapter):
```go
import (
    "forge.lthn.ai/core/go-inference"
    _ "forge.lthn.ai/core/go-mlx"  // registers "metal" backend via init()
)

func NewMLXBackend(modelPath string) (*InferenceAdapter, error) {
    m, err := inference.LoadModel(modelPath)
    if err != nil { return nil, fmt.Errorf("mlx: %w", err) }
    return &InferenceAdapter{model: m, name: "mlx"}, nil
}
```

All tokenisation, KV cache, sampling, and memory management is now handled inside go-mlx's `internal/metal/` package, accessed through the go-inference `TextModel` interface.

## Scoring Engine Architecture

### 5 Suites

| Suite | Method | LLM needed? | Metrics |
|-------|--------|-------------|---------|
| **Heuristic** | Regex + word analysis | No | 9 metrics → LEK composite |
| **Semantic** | LLM-as-judge | Yes | 4 dimensions (sovereignty, ethical, creative, self-concept) |
| **Content** | LLM-as-judge | Yes | 6 sovereignty probes (CCP, truth, engagement, etc.) |
| **Standard** | LLM-as-judge | Yes | TruthfulQA, DoNotAnswer, Toxigen |
| **Exact** | Numeric extraction | No | GSM8K answer matching |

### LEK Score Formula

```
LEK = EngagementDepth×2 + CreativeForm×3 + EmotionalRegister×2 + FirstPerson×1.5
    - ComplianceMarkers×5 - FormulaicPreamble×3 - Degeneration×4 - EmptyBroken×20
```

Positive signals: engagement depth, creative form, emotional register, first-person voice.
Negative signals: RLHF compliance markers, formulaic preambles, text degeneration, empty/broken output.

### Concurrency Model

`Engine.ScoreAll()` fans out goroutines bounded by semaphore (`concurrency` setting). Heuristic runs inline (instant). Semantic/content/standard run via worker pool with `sync.WaitGroup`. Results collected into `[]PromptScore` via mutex.

## Known Issues

- **backend_mlx.go imports dead subpackages** — Blocked on Phase 1 migration
- **agent.go too large** — 1,070 LOC, SSH + InfluxDB + scoring + publishing mixed together
- **Hardcoded infrastructure** — InfluxDB endpoint `10.69.69.165:8181`, M3 SSH details in agent.go
- **No tests for backend_llama and backend_mlx** — Only backend_http_test.go exists
- **score.go concurrency untested** — No race condition tests
- **Message type duplication** — `ml.Message` and `inference.Message` are identical but separate
