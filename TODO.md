# TODO.md — go-ml Task Queue

## Phase 1: Post-Split Hardening

- [ ] **Verify tests pass standalone** — Run `go test ./...`. Confirm all 6 test files pass (backend_http, exact, heuristic, judge, probes, score).
- [ ] **agent.go audit** — 1,070 LOC is the largest file. Review for decomposition opportunities. May benefit from splitting into agent_config.go, agent_execute.go, agent_eval.go.
- [ ] **Backend interface docs** — Add godoc examples showing how to implement a custom Backend.

## Phase 2: Scoring Improvements

- [ ] **Benchmark scoring suites** — No benchmarks exist. Add: BenchmarkHeuristic, BenchmarkJudge, BenchmarkExact for various input sizes.
- [ ] **Probe coverage** — Audit probes.go for completeness against OWASP LLM Top 10 and ethics guidelines.
- [ ] **Scoring pipeline metrics** — Track time-per-suite, pass/fail rates, aggregated scores over time.

## Phase 3: Backend Enhancements

- [ ] **Backend registry** — Currently backends are created ad-hoc. Add a registry pattern for discovery and configuration.
- [ ] **Health checks** — Backends should expose health status (model loaded, GPU available, API reachable).
- [ ] **Retry with backoff** — HTTP backend should retry on transient failures with exponential backoff.

---

## Workflow

1. Virgil in core/go writes tasks here after research
2. This repo's session picks up tasks in phase order
3. Mark `[x]` when done, note commit hash
