<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# heuristic.go — regex / programmatic scoring

**Package**: `dappco.re/go/ml`
**File**: `go/heuristic.go`

## What this is

The **fast lane** of the scoring engine — regex / programmatic checks that score a response without invoking an LLM. Used for:

- Format checks (does the response match the expected shape?)
- Forbidden-content checks (regex blocklist)
- Length / structure checks
- Domain-specific patterns (code compiles? JSON parses? expected keys present?)

Runs in microseconds per prompt. No model call, no judge call, no network.

## Heuristic

```go
type Heuristic struct {
    ID         string                                // identity in the report
    Description string
    Pattern    *regexp.Regexp                       // for regex-mode
    Check      func(prompt, response string) float64  // for custom logic
    Weight     float64                               // weight in suite-aggregate score
    Pass       float64                               // threshold for pass/fail
}
```

Either `Pattern` (for regex) or `Check` (for arbitrary Go logic) — not both.

## Suite

```go
heuristics := []ml.Heuristic{
    {ID: "no-html-in-prose", Pattern: regexp.MustCompile(`<\w+>`), Pass: 0.0},
    {ID: "min-length",      Check: minLengthCheck(50),             Pass: 1.0},
    {ID: "ends-with-period", Pattern: regexp.MustCompile(`\.\s*$`), Pass: 1.0},
    // …
}

suite := ml.NewHeuristicSuite(heuristics)
```

Score per heuristic: regex matches → 0.0 / 1.0, custom check → caller-defined. Weights aggregate to a per-suite score.

## Why this exists

Three suites of reasons:

1. **Cost.** A heuristic suite of 20 checks against 1000 prompts is milliseconds; the same in semantic-judge mode is hours of LLM time.
2. **Reproducibility.** Regex output is deterministic; LLM-judge isn't.
3. **CI integration.** Heuristic checks can run on every PR; LLM-judge checks gate big merges.

## Limitations

Can't capture:

- Factual accuracy (no LLM in the loop)
- Tone / register (subtle semantic dimensions)
- Multi-step reasoning correctness

For those, the engine falls back to `judge`-using suites.

## Used by

- `Engine.ScoreAll` ([score.md](score.md)) — heuristic suite is on by default
- CI eval runs — fast gate before the expensive suites
- Quick local sanity checks during dev

## Related

- [score.md](score.md) — engine
- [judge.md](judge.md) — LLM-judge complement
- [exact.md](exact.md) — sibling for exact-match
- [probes.md](probes.md) — sibling for content/sovereignty
