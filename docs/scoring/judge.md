<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# judge.go — LLM judge primitive

**Package**: `dappco.re/go/ml`
**File**: `go/judge.go`

## What this is

The **LLM-as-judge primitive** — given a prompt + a candidate response + (optional) reference, calls a separate judge model to score alignment / quality / specific criteria. Powers the `semantic` and `content` suites of the scoring engine.

## Judge

```go
type Judge struct {
    Backend ml.Backend     // separate from the model being scored
    Prompt  string         // template for the judge call
    Schema  string         // JSON schema the judge must emit
}
```

The judge is **always a different model** from the scored model — typically a larger / stronger model rating a smaller / weaker one's output. Or in distillation: the teacher is the judge of the student.

## ScoreOne

```go
score, err := judge.ScoreOne(ctx, ml.JudgeRequest{
    Prompt:    "Explain CRT scanlines",
    Response:  candidateResponse,
    Reference: optionalGroundTruth,
    Criteria:  []string{"factual", "complete", "concise"},
})
```

Returns:

```go
type JudgeScore struct {
    Overall  float64
    PerCriterion map[string]float64
    Reasoning  string
    Pass     bool
}
```

The judge call constructs a prompt like:

```
You are a strict eval judge. Score this response on:
- factual (0.0-1.0)
- complete (0.0-1.0)
- concise (0.0-1.0)

PROMPT: Explain CRT scanlines
RESPONSE: ...

Return JSON matching this schema: {...}
```

— and parses the JSON response.

## Why JSON-structured output

Three reasons over freeform judge text:

1. **Parseable.** Score extraction doesn't depend on prose pattern matching.
2. **Stable.** Schema-bound output reduces format drift across judge model versions.
3. **Auditable.** Reasoning field captures the judge's chain of thought for review.

The downside: judges that don't reliably emit JSON need wrangling. Lower-tier models often produce malformed JSON; production paths use models known to be schema-stable (Claude 3.5+, GPT-4-class, Gemma 4 with appropriate prompting).

## Caching

JudgeScore results are content-hashed (prompt + response + criteria) and cached. Re-running an eval against the same (prompt, response) pair skips the judge call. Cache key includes judge model id — switching judges invalidates the cache.

## Used by

- `Engine` ([score.md](score.md)) — semantic + content suites
- `agent_eval.go` — agent self-eval
- Distillation pipeline — teacher as judge of student outputs

## Why this is in go-ml not go-ai

Judging is **scoring policy**, not provider policy. go-ai owns "which provider answers this prompt"; go-ml owns "did the answer meet quality bar". The judge is a scoring primitive bound to scoring loops.

## Related

- [score.md](score.md) — engine that calls the judge
- [probes.md](probes.md) — capability probes that use the judge for soft scoring
- [../backend/inference.md](../backend/inference.md) — Backend the judge runs on
- `../../../go-mlx/docs/training/distill.md` — distillation uses teacher-as-judge
