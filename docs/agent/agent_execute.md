<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# agent_execute.go — single-checkpoint execution helpers

**Package**: `dappco.re/go/ml`
**File**: `go/agent_execute.go`

## What this is

The **one-shot execution** helpers used by the agent for tasks that don't fit the "discover → eval → emit" loop:

- Manual single-checkpoint eval (`agent eval-ckpt <path>`)
- Comparison runs ("score this checkpoint against this prompt set")
- Pre-flight validation ("is this checkpoint loadable?")
- Adapter-swap testing ("swap to this adapter, score, swap back")

Each is a function that fits into the surrounding `Agent` shape without spinning up the full polling loop.

## Surface

```go
agent.ExecuteEval(ctx, ckpt) (*ScoringReport, error)
agent.ExecuteComparison(ctx, ckpts []*Checkpoint, prompts) ([]*ScoringReport, error)
agent.ExecutePreflight(ctx, ckpt) error               // can this load + run?
agent.ExecuteAdapterSwap(ctx, oldA, newA, prompts) (...)
```

## Why a separate file

The loop case (`agent.go`) is the production path. The one-shot cases need different ergonomics — bypass polling, bypass watermarks, take a specific checkpoint or set. Putting them in the same file as the loop confused the responsibility boundaries.

## Used by

- CLI `core ml agent eval` — manual eval invocation
- LARQL — runs custom pre/post compare via this surface
- Test fixtures — drive eval logic without spinning the loop
- Vi training human-in-loop interventions

## Related

- [agent.md](agent.md) — orchestrator
- [agent_eval.md](agent_eval.md) — eval logic these wrap
- [agent_ssh.md](agent_ssh.md) — transport for fetching pieces
