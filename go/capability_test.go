// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"

	"dappco.re/go"
	"dappco.re/go/inference"
)

type capabilityTestBackend struct {
	name      string
	available bool
	report    inference.CapabilityReport
}

func (backend *capabilityTestBackend) Capabilities() inference.CapabilityReport {
	return backend.report
}

func (backend *capabilityTestBackend) Generate(_ context.Context, prompt string, _ GenOpts) core.Result {
	return core.Ok(newResult(prompt, nil))
}

func (backend *capabilityTestBackend) Chat(_ context.Context, messages []Message, _ GenOpts) core.Result {
	if len(messages) == 0 {
		return core.Ok(newResult("", nil))
	}
	return core.Ok(newResult(messages[len(messages)-1].Content, nil))
}

func (backend *capabilityTestBackend) Name() string { return backend.name }

func (backend *capabilityTestBackend) Available() bool { return backend.available }

func TestCapability_CapabilityReportForBackend_Good(t *core.T) {
	backend := &capabilityTestBackend{
		name:      "mlx",
		available: true,
		report: inference.CapabilityReport{
			Runtime:   inference.RuntimeIdentity{Backend: "metal", NativeRuntime: true},
			Available: true,
			Capabilities: []inference.Capability{
				inference.SupportedCapability(inference.CapabilityGenerate, inference.CapabilityGroupModel),
				inference.SupportedCapability(inference.CapabilityProbeEvents, inference.CapabilityGroupProbe),
			},
		},
	}

	report := CapabilityReportForBackend("mlx", backend)

	core.AssertEqual(t, "metal", report.Runtime.Backend)
	core.AssertTrue(t, report.Supports(inference.CapabilityGenerate))
	core.AssertTrue(t, report.Supports(inference.CapabilityProbeEvents))
}

func TestCapability_CapabilityReportForBackend_BadNil(t *core.T) {
	report := CapabilityReportForBackend("missing", nil)

	core.AssertEqual(t, "missing", report.Runtime.Backend)
	core.AssertFalse(t, report.Available)
	core.AssertLen(t, report.Capabilities, 0)
}

func TestCapability_CapabilityReportForBackend_UglyFallback(t *core.T) {
	backend := &capabilityTestBackend{name: "http", available: true}

	report := CapabilityReportForBackend("", backend)

	core.AssertEqual(t, "http", report.Runtime.Backend)
	core.AssertTrue(t, report.Available)
	core.AssertTrue(t, report.Supports(inference.CapabilityGenerate))
	core.AssertTrue(t, report.Supports(inference.CapabilityChat))
}

func TestCapability_ServiceBackendCapabilities_Good(t *core.T) {
	svc := newServiceForTest(t, Options{})
	svc.RegisterBackend("local", &capabilityTestBackend{name: "local", available: true})

	report, ok := svc.BackendCapabilities("local")

	core.AssertTrue(t, ok)
	core.AssertEqual(t, "local", report.Runtime.Backend)
	core.AssertTrue(t, report.Supports(inference.CapabilityGenerate))
}

func TestCapability_InferenceAdapterCapabilities_Good(t *core.T) {
	model := &mockTextModel{modelType: "qwen3"}
	adapter := NewInferenceAdapter(model, "mlx")

	report := adapter.Capabilities()

	core.AssertTrue(t, report.Available)
	core.AssertEqual(t, "mlx", report.Runtime.Backend)
	core.AssertTrue(t, report.Supports(inference.CapabilityGenerate))
	core.AssertTrue(t, report.Supports(inference.CapabilityChat))
}

func TestCapability_InferenceAdapterCapabilities_UglyNil(t *core.T) {
	var adapter *InferenceAdapter
	report := adapter.Capabilities()

	core.AssertEqual(t, "", report.Runtime.Backend)
	core.AssertFalse(t, report.Available)
}
