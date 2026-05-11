<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# exact.go — exact-match scoring

**Package**: `dappco.re/go/ml`
**File**: `go/exact.go`

## What this is

Exact-match scoring — string equality (with normalisation) between a model's response and a reference target. The simplest scoring suite; used for:

- Single-token classification ("yes" / "no")
- Multiple-choice answers ("A", "B", "C", "D")
- Structured outputs where exact-match is the only valid bar (extract a date, ID, name)
- Regression tests where a known-good response must reproduce

## Match modes

```go
type ExactMatchOptions struct {
    CaseInsensitive bool
    TrimWhitespace  bool
    NormaliseSpaces bool   // collapse multiple whitespace to single
    Accent          string // "fold" | "preserve"
}
```

The defaults are conservative — case-sensitive, no normalisation. Callers tune per task domain.

## Levenshtein fallback (planned)

For tasks where "almost-exact" should still score, an optional Levenshtein distance fallback returns a continuous score (0.0 = identical, 1.0 = max distance). Off by default; opt-in per suite.

## Used by

- Classification eval (TruthfulQA option matching)
- Multi-choice benchmarks (GSM8K final-answer extraction)
- Regression tests (golden response equality)
- Distillation eval (compare student output to teacher token-by-token in batch mode)

## Why this lives in scoring not in inference

`inference.Classify` returns a `Token` — token-id and text. That's the runtime primitive. Comparing the response text against an expected string is *scoring* — it interprets the meaning of the token. Different layer.

## Related

- [score.md](score.md) — engine that runs this suite
- [heuristic.md](heuristic.md) — fast lane sibling
- [judge.md](judge.md) — semantic fallback for cases where exact-match misses
- `../../../go-inference/docs/inference/inference.md` — `Classify` interface upstream
