<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# agent_eval.go — agent's eval logic

**Package**: `dappco.re/go/ml`
**File**: `go/agent_eval.go`

## What this is

The **eval logic** wired into the agent loop ([agent.md](agent.md)). Given a freshly discovered checkpoint, runs evaluation suites against it and emits a `ScoringReport`.

Splits the work that's specific to "evaluating a remote checkpoint" from the generic scoring engine — pulls metadata, loads the adapter against a configured base, dispatches to the engine, post-processes results.

## Surface

```go
func (a *Agent) EvalCheckpoint(ctx, ckpt *Checkpoint) (*ScoringReport, error)
```

Internal flow:

1. Pull checkpoint metadata via transport
2. Load the base model + this checkpoint's adapter into a backend
3. Build a `ScoringPrompt` set from configured prompt source
4. Call `engine.ScoreAll(ctx, prompts)`
5. Stamp the report with checkpoint identity + step
6. Return

## Result decoration

Pre-engine: prompt set is fixed per agent config — same prompts for every checkpoint so trends are comparable.

Post-engine: adds:
- `Checkpoint.Path`, `Step`, `BaseModel`, `Adapter`
- Time of evaluation
- Agent name
- Engine config snapshot (which suites, which judge)

The decorated report is what InfluxDB ingests.

## Adapter swap

The agent maintains one warm backend per base model. For each new checkpoint:

```
detach current adapter
attach new adapter
score
detach new adapter
(optionally) re-attach previous (cheap rollback)
```

Hot-swapping the adapter on a loaded base avoids the full-model reload cost per checkpoint — typically ~200ms per checkpoint vs ~30s if full-reloading.

## Why split from agent.go

`agent.go` owns the **loop and config**. `agent_eval.go` owns the **scoring work**. Two distinct mental models:

- Loop logic: when to poll, when to stop, hook ordering
- Eval logic: how to score, how to interpret a checkpoint, what report to emit

Keeps each file under 300 lines.

## Used by

- `Agent.Run()` — call point in the loop
- Direct invocation by tests / scripts that want one-shot eval

## Related

- [agent.md](agent.md) — orchestrator that calls this
- [agent_execute.md](agent_execute.md) — single-execution helpers
- [agent_ssh.md](agent_ssh.md) — pulls the checkpoint metadata
- [agent_influx.md](agent_influx.md) — destination for the decorated report
- [../scoring/score.md](../scoring/score.md) — engine
