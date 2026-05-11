<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# agent_influx.go — InfluxDB result streaming

**Package**: `dappco.re/go/ml`
**File**: `go/agent_influx.go`

## What this is

The InfluxDB write path for agent eval results. Each `ScoringReport` becomes a line-protocol record (or batch) pushed to InfluxDB for dashboarding + alerting.

## Why InfluxDB

Three reasons over Postgres / DuckDB / JSONL:

1. **Time-series shape.** Per-checkpoint reports are naturally time-indexed; InfluxDB's retention + downsampling fits.
2. **Grafana integration.** Dashboards consume InfluxDB out-of-the-box. No bespoke ETL.
3. **Alert rules.** "If score drops 5% in 24h, alert" lives in InfluxDB tasks, not in the agent.

## Schema

Each report writes one measurement: `ml_eval`

Tags (indexed):
- `agent` — agent name
- `model` — base model id
- `adapter` — adapter id
- `suite` — scoring suite name
- `backend` — backend name

Fields (numeric):
- `score` — suite score (0.0-1.0)
- `prompt_count` — N prompts evaluated
- `duration_ms` — eval wall clock
- `pass` — 0 or 1

Timestamp: report's `StartedAt`.

One point per suite per report. A 5-suite report writes 5 points.

## Buffering

The Influx writer buffers up to N points (default 100) and flushes on interval (default 10s) or on agent stop. Avoids per-checkpoint HTTP round-trip overhead while keeping the dashboard fresh.

Buffer overflow on outage: drops oldest points with a logged warning. Not a durable queue — checkpoints surveyed pre-outage that didn't get written are lost from Influx (the per-checkpoint JSONL artefact remains).

## Reads

The agent doesn't read from InfluxDB. Reads come from Grafana / external tools / the IDE dashboard. This file is write-only.

## Used by

- `Agent.Run()` — auto-emits each completed eval
- Direct invocation by ad-hoc scripts

## Related

- [agent.md](agent.md) — orchestrator that invokes this
- [agent_eval.md](agent_eval.md) — produces the reports being written
- [../scoring/score.md](../scoring/score.md) — report shape
- Vi training dashboard (planned) — consumes Influx data
