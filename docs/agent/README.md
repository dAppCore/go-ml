<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# agent/ — agent orchestrator

**Package**: `dappco.re/go/ml` (these files live in the root)

## What this area owns

The **agent orchestrator** — discovers fine-tune checkpoints on a remote box, evaluates each, streams results to InfluxDB. The eval pipeline that runs alongside Vi training and the distillation cascade.

| File | Doc | Role |
|------|-----|------|
| `agent.go` | [agent.md](agent.md) | Orchestrator core (loop, config wiring) |
| `agent_config.go` | [agent_config.md](agent_config.md) | YAML / TOML config parsing |
| `agent_eval.go` | [agent_eval.md](agent_eval.md) | Per-checkpoint eval logic |
| `agent_execute.go` | [agent_execute.md](agent_execute.md) | One-shot execution helpers |
| `agent_ssh.go` | [agent_ssh.md](agent_ssh.md) | SSH RemoteTransport |
| `agent_influx.go` | [agent_influx.md](agent_influx.md) | InfluxDB result writer |

## Mental model

```
              ┌───────────────────────┐
              │  AgentConfig          │ ← config file
              └──────────┬────────────┘
                         │
                         ▼
   ┌──────────────────────────────────────────┐
   │           Agent loop                       │
   │  ┌─────────────────────────────────────┐  │
   │  │ Transport.ListNew(dir, watermark)   │  │
   │  │   → []checkpoint paths               │  │
   │  └─────────────┬───────────────────────┘  │
   │                ▼                            │
   │  ┌─────────────────────────────────────┐  │
   │  │ for each new ckpt:                   │  │
   │  │   ExecuteEval (engine.ScoreAll)      │  │
   │  │   write to Influx                    │  │
   │  └─────────────────────────────────────┘  │
   │                ▼                            │
   │           Sleep(PollInterval)               │
   └──────────────────────────────────────────┘
```

## Why an orchestrator vs ad-hoc scripts

Three reasons:

1. **Continuous.** Training runs continue overnight; the agent picks up checkpoints as they land instead of operator polling.
2. **Reproducible.** Same prompt set, same suites, same judge across all checkpoints in a run → trends are comparable.
3. **Audit.** Influx + JSONL audit retain who-tested-what-when without operator log scraping.

## Related

- [../backend/](../backend/README.md) — backends the agent uses for eval
- [../scoring/](../scoring/README.md) — engine the agent runs
- `../../../go-mlx/docs/training/sft.md` — produces the checkpoints
- `../../../go-mlx/docs/training/distill.md` — distillation produces them too
- `project_vi_training_plan.md` — Vi pipeline that uses this agent
