<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# agent_config.go — agent config parsing + validation

**Package**: `dappco.re/go/ml`
**File**: `go/agent_config.go`

## What this is

The **config parsing layer** for the agent. Reads a YAML or TOML config file (configurable), validates it, builds the typed `AgentConfig` the orchestrator consumes.

Small but load-bearing: the agent's behaviour is entirely driven by this config, so misconfigurations need clear early-fail errors.

## Surface

```go
cfg, err := ml.LoadAgentConfig("/etc/ml-agent.yaml")
agent := ml.NewAgent(cfg)
```

## What gets validated

- Transport credentials present (SSH key file readable, etc.)
- CheckpointDir non-empty
- Engine config produces a valid Engine (suites list valid, judge backend reachable if needed)
- Poll interval ≥ minimum threshold (avoid hammering)
- Concurrency ≥ 1
- InfluxClient config valid if Influx writer configured

## Error model

Errors fail-fast at load — the agent doesn't start if config is invalid. Better to refuse to start than to start and silently miss checkpoints.

Error messages include the YAML/TOML path that failed (`"transport.private_key_path: cannot read /etc/ml-agent-key: permission denied"`) so operators can fix without grepping the code.

## Default config

A `DefaultAgentConfig()` builds a minimal working config for local dev:

- Transport: local-filesystem (no SSH)
- CheckpointDir: `./checkpoints`
- Poll interval: 60s
- Concurrency: 1
- No Influx writer
- Heuristic suite only (fast lane)

## Used by

- `cmd/lem agent` CLI — load config from `--config` flag
- Tests building synthetic configs
- The Service layer when registering as a Core service

## Related

- [agent.md](agent.md) — consumer of the AgentConfig
- [agent_ssh.md](agent_ssh.md) — Transport configured here
- [agent_influx.md](agent_influx.md) — Influx config sub-field
