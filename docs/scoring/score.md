<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# score.go — multi-suite scoring engine

**Package**: `dappco.re/go/ml`
**File**: `go/score.go`

## What this is

The **scoring engine** — runs a model's responses against a configured set of suites (heuristic, semantic, content, standard benchmarks, exact-match) and emits a structured score report. Used as the eval shape for fine-tune checkpoints, regression testing, and "is the new model better?" comparisons.

## Engine

```go
engine := ml.NewEngine(ml.EngineConfig{
    Backend: backend,                  // ml.Backend to score
    Judge:   judge,                    // ml.Judge for semantic scoring
    Suites:  "all",                    // or "heuristic,semantic,exact"
    Workers: 4,                        // concurrency cap
})

report := engine.ScoreAll(ctx, prompts)
```

## Suites

| Suite | What it scores | Cost |
|-------|----------------|------|
| **heuristic** | regex / programmatic checks ([heuristic.md](heuristic.md)) | fast (no LLM) |
| **semantic** | LLM-judge alignment ([judge.md](judge.md)) | judge inference per prompt |
| **content** | sovereignty / ethics probes ([probes.md](probes.md)) | judge inference |
| **standard** | TruthfulQA, DoNotAnswer, Toxigen, GSM8K | dataset-specific |
| **exact** | string equality on generated vs target ([exact.md](exact.md)) | fast |

`Suites` accepts a comma-separated list or `"all"`. Engine selects which to run at construction; cost scales with judge-using suites.

## ScoreAll

```go
report := engine.ScoreAll(ctx, []ScoringPrompt{
    {ID: "p1", Prompt: "Explain CRT scanlines", Target: "..."},
    {ID: "p2", ...},
})
```

Fans out across configured suites concurrently with a semaphore-bounded worker pool. Returns a `ScoringReport` with per-prompt + per-suite scores + aggregate stats.

## ScoringReport

```go
type ScoringReport struct {
    Backend     string
    Model       string
    Suites      []string
    Prompts     []PromptResult
    Aggregates  map[string]float64    // mean per suite
    Pass        bool                  // overall pass/fail
    StartedAt   time.Time
    DurationMs  int64
}

type PromptResult struct {
    ID         string
    Response   string
    Scores     map[string]float64    // per-suite
    Notes      []string
}
```

JSON-serialisable end-to-end — easy to compare reports across runs, easy to drop into the audit pipeline.

## Concurrency

Worker pool sized by `Workers` config. Each worker pulls from a prompt channel, runs all configured suites for that prompt, pushes results. Bounded so the underlying backend doesn't get hammered beyond its scheduler's MaxConcurrent.

## Why "all in one engine"

Three reasons over per-suite runner functions:

1. **Shared backend / shared judge.** One backend instance, one judge instance, many suites — minimises model load + warm cost.
2. **Per-prompt rollup.** Returns one report per prompt with all suite scores — easier downstream analysis than N reports to join.
3. **Pass/fail aggregation.** A configurable rule decides overall pass: typically "all critical suites ≥ threshold". One place to express it.

## Used by

- `cmd/lem` `core ml score` — CLI eval entry
- `agent_eval.go` — agent's pre-/post-finetune comparison
- `api/routes.go` — `/v1/ml/score` HTTP endpoint
- Vi training pipeline — checkpoint eval gate

## Related

- [judge.md](judge.md) — semantic scoring
- [heuristic.md](heuristic.md) — fast regex/programmatic
- [exact.md](exact.md) — string equality
- [probes.md](probes.md) — content/sovereignty probes
- [../backend/inference.md](../backend/inference.md) — Backend being scored
- `../../../go-inference/docs/inference/training.md` — feeds checkpoints into the engine
