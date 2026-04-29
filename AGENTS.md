# go-ml Agent Guide

This repository is the machine-learning lane for the DappCore Go stack. It
contains model backend adapters, scoring engines, data import/export tools,
agent orchestration, CLI commands, API routes, and MCP integration for local
and remote ML evaluation workflows.

Use `dappco.re/go` primitives for formatting, JSON, paths, files, assertions,
and errors. Direct imports of the wrapped standard-library packages are not
accepted in production code or tests. Tests use the local source-file shape:
for each public symbol in `foo.go`, keep its `Good`, `Bad`, and `Ugly` cases
in `foo_test.go`, and keep runnable examples in `foo_example_test.go`.

The main package is `ml`. The `cmd` package wires the CLI surface, `api`
exposes Gin route groups, and `pkg/mcp` exposes the MCP subsystem. Avoid
moving tests into aggregate compliance files; source ownership is intentionally
visible from the sibling test and example files.

Before handing off changes, run the v0.9.0 audit together with the normal Go
verification commands described in `BRIEF.md`. The audit is the source of
truth for repository compliance.
