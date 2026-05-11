<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# go-ml — documentation index

**Module**: `dappco.re/go/ml`
**Role**: Scoring engine, eval orchestration, and agent loop. Consumes `inference.TextModel` from native backends; produces structured scoring reports + InfluxDB streams.

## Tetrad position

```
                    ┌──────────────────────────────┐
                    │      dappco.re/go (core)     │
                    └──────────────┬───────────────┘
                                   │
                    ┌──────────────┴────────────────┐
                    │     go-inference  (contract)  │
                    └──┬─────────────┬──────────────┘
                       │             │ register via init()
              ┌────────┴───┐  ┌──────┴────────┐
              │  go-mlx    │  │  go-rocm /    │
              └─────┬──────┘  └───────────────┘
                    │ consumed by
        ┌───────────┴────────┬─────────────────┐
        │   you are here →   │  go-ai           │
        │   go-ml            │  router/demos    │
        │   scoring + agent  │                  │
        └────────────────────┘ └────────────────┘
```

## Doc tree

```
docs/
├── README.md             ← you are here
├── backend/              ← ml.Backend impls + adapters
│   ├── README.md
│   ├── inference.md      — ml.Backend interface
│   ├── adapter.md        — InferenceAdapter (go-inference → ml)
│   ├── backend_http.md   — OpenAI-compatible HTTP backend
│   ├── backend_llama.md  — managed llama-server subprocess
│   ├── backend_mlx.md    — go-mlx convenience entry
│   └── capability.md     — CapabilityReport bridge
│
├── scoring/              ← scoring engine + suites
│   ├── README.md
│   ├── score.md          — Engine, ScoringReport, fan-out logic
│   ├── judge.md          — LLM-as-judge primitive
│   ├── heuristic.md      — regex / programmatic
│   ├── exact.md          — exact string match
│   └── probes.md         — 23 capability probes
│
└── agent/                ← orchestrator
    ├── README.md
    ├── agent.md          — orchestrator core
    ├── agent_config.md   — config parsing
    ├── agent_eval.md     — per-checkpoint eval logic
    ├── agent_execute.md  — one-shot helpers
    ├── agent_ssh.md      — SSH transport
    └── agent_influx.md   — InfluxDB writer
```

## Where to start

- **"How does scoring work?"** → [`scoring/score.md`](scoring/score.md)
- **"What backends does go-ml know about?"** → [`backend/README.md`](backend/README.md)
- **"How do I attach a custom heuristic?"** → [`scoring/heuristic.md`](scoring/heuristic.md)
- **"How does the eval-checkpoints loop work?"** → [`agent/agent.md`](agent/agent.md)
- **"Why are there two `Backend` interfaces?"** → [`backend/inference.md`](backend/inference.md) (and [`backend/adapter.md`](backend/adapter.md))

## What's in this module

| Path | Purpose |
|------|---------|
| `go/*.go` | root package (Backend impls, scoring, agent) |
| `go/api/` | REST API exposing `/v1/ml/*` |
| `go/cmd/` | CLI entry — `core ml ...` subcommands |

## Recent change context

The bulk of the dirty diff on this branch is a **mechanical sweep** replacing `coreerr.E(scope, msg, err)` (from the old `dappco.re/go/log` package) with `core.E(scope, msg, err)` (from the unified `dappco.re/go`). 83 files, 231 insertions vs 237 deletions of the same call shape. No behaviour change — just unifying the error builder.

## Standards

- UK English (colour, organisation, centre, licence)
- SPDX header: `// SPDX-Licence-Identifier: EUPL-1.2`
- Error wrapping via `core.E(scope, msg, cause)` — never `fmt.Errorf` or panic
- Test triplets: `_Good` / `_Bad` / `_Ugly`
- Conventional commits scoped to `backend`, `scoring`, `probes`, `agent`, `service`, `types`, `gguf`
- Co-Author: `Co-Authored-By: Virgil <virgil@lethean.io>`
- `-tags nomlx` to build without the Metal backend
