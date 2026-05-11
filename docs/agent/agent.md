<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# agent.go — agent orchestrator core

**Package**: `dappco.re/go/ml`
**File**: `go/agent.go`

## What this is

The **agent orchestrator** for fine-tune checkpoint discovery, eval scheduling, and result streaming. Sits above the scoring engine ([score.md](../scoring/score.md)) — knows about training runs, remote checkpoint storage, InfluxDB result streams, and the agent's decision loop.

Used in the "I trained Vi overnight, was it any good?" workflow: discover new checkpoints on the homelab M3, eval each one, stream results to InfluxDB, surface in dashboards.

## AgentConfig

```go
type AgentConfig struct {
    Name            string                  // agent identifier
    Transport       RemoteTransport          // typically SSH
    CheckpointDir   string                  // remote path where checkpoints land
    PollInterval    time.Duration
    Engine          *ml.Engine              // for evaluation
    Influx          *InfluxClient           // result destination
    SuiteFilter     []string                // which scoring suites to run
    Concurrency     int
    OnCheckpoint    func(*Checkpoint) error // hook on discovery
    OnEvalResult    func(*ScoringReport) error
}
```

## Agent

```go
agent := ml.NewAgent(cfg)
err := agent.Run(ctx)        // blocks until ctx cancelled
agent.Stop()                  // alternative to ctx cancellation
```

Loops:

1. Poll `CheckpointDir` via Transport for new files
2. For each new checkpoint:
   - Pull metadata
   - Run `Engine.ScoreAll` against configured prompts
   - Emit `ScoringReport` via OnEvalResult + InfluxDB
   - Mark seen so next poll skips it
3. Sleep `PollInterval`, repeat

## Checkpoint

```go
type Checkpoint struct {
    Path       string                  // absolute path on remote
    Step       int
    BaseModel  string
    Adapter    string                  // adapter file path
    CreatedAt  time.Time
    Metadata   map[string]string
}
```

Constructed by inspecting the checkpoint file (sft / grpo / distill metadata version + step count + base model identity).

## RemoteTransport

```go
type RemoteTransport interface {
    ListNew(ctx, dir, since time.Time) ([]string, error)
    Read(ctx, path) ([]byte, error)
    Stat(ctx, path) (FileInfo, error)
}
```

Implementations:

- `SSHTransport` ([agent_ssh.md](agent_ssh.md)) — production path
- in-memory fake for tests

## Why a separate file from agent_eval / agent_execute / agent_ssh

`agent.go` is the **orchestrator** — config, lifecycle, top-level loop. The sibling files own:

- `agent_eval.go` — what evaluation actually consists of (calls Engine.ScoreAll)
- `agent_execute.go` — single-checkpoint execution helpers
- `agent_ssh.go` — SSH transport
- `agent_influx.go` — InfluxDB result streaming
- `agent_config.go` — config parsing / validation

Each is focused enough to be tested + reasoned about independently.

## Used by

- Vi training pipeline — eval every checkpoint as it appears on the homelab
- LARQL inspection — agent runs vindex extraction post-fine-tune
- Distillation cascade — auto-eval the student at every step

## Related

- [agent_eval.md](agent_eval.md) — eval logic
- [agent_ssh.md](agent_ssh.md) — transport
- [../scoring/score.md](../scoring/score.md) — engine the agent runs
- `../../../go-mlx/docs/training/sft.md` — produces the checkpoints the agent watches
