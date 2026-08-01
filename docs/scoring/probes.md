<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# probes.go — 23 capability probes

**Package**: `dappco.re/go/ml`
**File**: `go/probes.go`

## What this is

The **23 capability probes** — binary pass/fail tests across math, logic, reasoning, code, and word-problem categories. Each probe is a single prompt with a programmatic check or schema-bound judge call. Used to characterise a model's strengths/weaknesses in one report run.

Quick, repeatable, deterministic enough to be regression-trackable across fine-tune iterations.

## Probe shape

```go
type Probe struct {
    ID          string             // stable identifier
    Category    string             // "math" | "logic" | "reasoning" | "code" | "word"
    Prompt      string
    Check       func(response string) bool      // programmatic
    Judge       *JudgeRequest                   // alternative: judge-based
    DifficultyTier string                       // "easy" | "medium" | "hard"
}
```

Either `Check` or `Judge` populated — programmatic is preferred (deterministic + cheap), judge as fallback for semantic-only criteria.

## Categories

| Category | Probes | What they test |
|----------|--------|----------------|
| math | 5 | arithmetic, fractions, percentages, units, order of ops |
| logic | 4 | syllogism, contradiction, transitive inference, exclusion |
| reasoning | 5 | multi-step deduction, causal, hypothetical, abductive, planning |
| code | 5 | parse simple snippet, identify bug, suggest fix, refactor, name the output |
| word | 4 | analogy, antonym, definition match, idiom interpretation |

## Report shape

```go
type ProbeReport struct {
    Backend    string
    Model      string
    Probes     []ProbeResult
    Categories map[string]float64    // per-category pass rate
    Overall    float64
    DurationMs int64
}

type ProbeResult struct {
    ID       string
    Category string
    Pass     bool
    Response string
    Notes    string
}
```

## Why 23 not 100

Three tradeoffs:

1. **Speed.** 23 probes run in seconds against a fast backend; 100 would take minutes.
2. **Signal density.** Each probe maps to one capability axis. Adding more probes adds redundancy unless they're genuinely new axes.
3. **Stable ground truth.** Each probe's answer is known to the dev who wrote it. Scaling to 100+ means reviewers can't trust the ground truth at a glance.

The 23 cover enough to track checkpoint-to-checkpoint regressions without overrunning CI budget.

## Used by

- `Engine.ScoreAll` ([score.md](score.md)) — `content` suite
- `agent_eval.go` — pre-/post-finetune snapshot
- Vi training checkpoint validation

## Related

- [score.md](score.md) — engine
- [judge.md](judge.md) — alternative scoring for the judge-mode probes
- `../../../go-mlx/docs/training/eval.md` — full-dataset eval (the bigger sibling)
- `project_8pac_eval_methodology.md` — Lethean eval methodology
