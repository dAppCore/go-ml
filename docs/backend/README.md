<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# backend/ — ml.Backend implementations

**Package**: `dappco.re/go/ml` (these files live in the root)

## What this area owns

The **backend layer** — how go-ml gets text out of a model. Four concrete backends + an adapter that turns any `inference.TextModel` into one:

| File | Doc | Backend type |
|------|-----|--------------|
| `inference.go` | [inference.md](inference.md) | `Backend` interface |
| `adapter.go` | [adapter.md](adapter.md) | `InferenceAdapter` — wraps `inference.TextModel` |
| `backend_http.go` | [backend_http.md](backend_http.md) | HTTP / OpenAI-compatible endpoint |
| `backend_http_textmodel.go` | (planned) | reverse adapter (HTTP → TextModel) |
| `backend_llama.go` | [backend_llama.md](backend_llama.md) | managed llama-server subprocess |
| `backend_mlx.go` | [backend_mlx.md](backend_mlx.md) | go-mlx convenience entry |
| `capability.go` | [capability.md](capability.md) | report bridge to `inference.CapabilityReport` |
| `ollama.go` | (planned) | Ollama-native API |

## Dual-interface design

```
inference.TextModel ──► InferenceAdapter ──► ml.Backend
ml.HTTPBackend     ──► HTTPTextModel    ──► inference.TextModel
```

Two interfaces (`inference.TextModel` vs `ml.Backend`) coexist with adapters in both directions. Use whichever fits the caller's shape — token-level streaming consumers go through `inference.TextModel`; buffered-text scoring/eval consumers go through `ml.Backend`.

## Selection logic

In production, the service starts up with a registered set of backends:

```
mlx (if darwin/arm64 + mlx available)
llama (if binary + model configured)
http (if endpoints configured)
ollama (if endpoint configured)
```

Consumers can request a backend by name or let `ml.Service.DefaultBackend()` pick.

## Related

- [../scoring/](../scoring/score.md) — primary backend consumer
- [../agent/](../agent/agent.md) — agent uses backends for eval
- `../../../go-inference/docs/inference/inference.md` — the contract package
