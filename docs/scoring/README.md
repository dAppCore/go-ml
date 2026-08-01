<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# scoring/ — the scoring engine

**Package**: `dappco.re/go/ml` (these files live in the root)

## What this area owns

The **scoring engine** + its suites. Given a model response, produce a structured score across configured dimensions.

| File | Doc | Suite type |
|------|-----|------------|
| `score.go` | [score.md](score.md) | Engine — runs N suites concurrently |
| `judge.go` | [judge.md](judge.md) | LLM-as-judge primitive |
| `heuristic.go` | [heuristic.md](heuristic.md) | Regex / programmatic |
| `exact.go` | [exact.md](exact.md) | Exact string match |
| `probes.go` | [probes.md](probes.md) | 23 capability probes (math/logic/reasoning/code/word) |

## Mental model

```
                ┌─────────────────────────────┐
                │     ScoringPrompt[]          │
                └──────────────┬──────────────┘
                               │
                               ▼
                ┌─────────────────────────────┐
                │   Backend.Generate / .Chat   │  (one model call per prompt)
                └──────────────┬──────────────┘
                               │
                               ▼
                ┌─────────────────────────────┐
                │  fan out across suites      │
                │  ├── heuristic (fast)        │
                │  ├── exact (fast)            │
                │  ├── probes (fast)           │
                │  ├── semantic ← judge call   │
                │  └── content  ← judge call   │
                └──────────────┬──────────────┘
                               │
                               ▼
                ┌─────────────────────────────┐
                │     ScoringReport            │
                └─────────────────────────────┘
```

Fast suites run inline; judge-using suites batch. Engine controls concurrency to keep both backend and judge under their respective rate limits.

## Suite selection

`EngineConfig.Suites` is comma-separated or `"all"`. Cheap suites can run on every PR (`heuristic,exact,probes`); expensive suites gate merges (`semantic,content`).

## Related

- [../backend/](../backend/README.md) — backends being scored
- [../agent/agent_eval.md](../agent/agent_eval.md) — agent consumer
- `../../../go-mlx/docs/training/eval.md` — full-dataset eval (the bigger sibling for training)
- `project_8pac_eval_methodology.md` — Lethean eval methodology
