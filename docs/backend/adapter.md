<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# adapter.go — InferenceAdapter (go-inference → ml.Backend)

**Package**: `dappco.re/go/ml`
**File**: `go/adapter.go`

## What this is

`InferenceAdapter` — bridges a `dappco.re/go/inference.TextModel` (iter.Seq token stream) into go-ml's `Backend` shape (buffered string + optional Metrics). The integration point that lets go-mlx, go-rocm, or any other native backend feed go-ml's scoring engine without converting interfaces at every call site.

## Construction

```go
import (
    "dappco.re/go/inference"
    "dappco.re/go/ml"
    _ "dappco.re/go/mlx"   // registers "metal"
)

r := inference.LoadModel("/models/gemma-4-e2b")
if !r.OK { return r }
model := r.Value.(inference.TextModel)

adapter := ml.NewInferenceAdapter(model, "mlx")   // implements ml.Backend
```

The `name` is what `adapter.Name()` returns — used by scoring reports to identify which backend produced a result.

## Methods (Backend implementation)

```go
adapter.Generate(ctx, prompt, GenOpts) core.Result      // r.Value = ml.Result
adapter.Chat(ctx, []Message, GenOpts) core.Result
adapter.Name() string
adapter.Available() bool
adapter.Model() inference.TextModel                     // unwrap (for callers needing iter.Seq)
adapter.Close() error
adapter.Capabilities() inference.CapabilityReport       // if model implements CapabilityReporter
adapter.InspectAttention(ctx, prompt, opts) core.Result // if model implements AttentionInspector
```

## Token buffering

`Generate` / `Chat` iterate the `iter.Seq[Token]` and concatenate into a string via `core.NewBuilder`. After exhausting the iterator, it captures `model.Metrics()` and `model.Err()`:

```go
for tok := range a.model.Generate(ctx, prompt, ...) {
    builder.WriteString(tok.Text)
}
if err := a.model.Err(); err != nil {
    return core.Fail(...)
}
metrics := a.model.Metrics()
return core.Ok(Result{Text: builder.String(), Metrics: &metrics})
```

No streaming through this adapter — the scoring engine wants the full response anyway. For streaming callers, use `adapter.Model()` to unwrap and consume `iter.Seq[Token]` directly.

## Optional capabilities exposed

`InspectAttention` is exposed when the wrapped model satisfies `inference.AttentionInspector`. Used by capability probes to inspect attention without binding to a specific backend.

`Capabilities()` is exposed via type-assertion to `inference.CapabilityReporter`. Returns the underlying model's claimed capabilities — go-ml's `CapabilityReportForBackend` ([capability.md](capability.md)) wraps this for ml-side reports.

## Why the adapter exists

Three reasons:

1. **API mismatch.** Scoring code wants a full string; native models give a token stream.
2. **Metrics integration.** ml's report shape needs metrics; the adapter passes them through.
3. **One Backend type.** Engine.ScoreAll iterates `[]Backend`; HTTP / llama / native all look the same to it.

## Used by

- `Service.OnStartup` (`service.go`) — register backends discovered at startup
- `Engine.ScoreAll` ([score.md](../scoring/score.md)) — backend-agnostic scoring
- `agent_eval.go` — eval against any backend
- Custom test fixtures wrapping mocked models

## Related

- [inference.md](inference.md) — `ml.Backend` interface
- [capability.md](capability.md) — capability reporting on top of this
- `../../../go-inference/docs/inference/inference.md` — `TextModel` being wrapped
- `../../../go-mlx/docs/runtime/adapter.md` — analogous adapter on the mlx side (server-side wrap)
