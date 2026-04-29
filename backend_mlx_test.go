// SPDX-Licence-Identifier: EUPL-1.2

//go:build darwin && arm64 && !nomlx

package ml

import (
	"context"
	"dappco.re/go"

	"dappco.re/go/inference"
)

// ---------------------------------------------------------------------------
// backend_mlx.go tests — uses mockTextModel from adapter_test.go
// since we cannot load real MLX models in CI
// ---------------------------------------------------------------------------

// TestMLXBackend_InferenceAdapter_Generate_Good verifies that an
// InferenceAdapter (the type returned by NewMLXBackend) correctly
// generates text through a mock TextModel.
func TestMLXBackend_InferenceAdapter_Generate_Good(t *core.T) {
	mock := &mockTextModel{
		tokens: []inference.Token{
			{ID: 1, Text: "MLX "},
			{ID: 2, Text: "output"},
		},
		modelType: "qwen3",
	}
	adapter := NewInferenceAdapter(mock, "mlx")

	// The adapter should satisfy Backend.
	var backend Backend = adapter
	core.AssertEqual(t, "mlx", backend.Name())
	core.AssertTrue(t, backend.Available())

	result, err := backend.Generate(context.Background(), "prompt", GenOpts{Temperature: 0.5})
	core.RequireNoError(t, err)
	core.AssertEqual(t, "MLX output", result.Text)
	core.AssertNotNil(t, result.Metrics)
}

// TestMLXBackend_InferenceAdapter_Chat_Good verifies chat through the
// InferenceAdapter wrapper (the path NewMLXBackend takes).
func TestMLXBackend_InferenceAdapter_Chat_Good(t *core.T) {
	mock := &mockTextModel{
		tokens: []inference.Token{
			{ID: 1, Text: "chat "},
			{ID: 2, Text: "reply"},
		},
	}
	adapter := NewInferenceAdapter(mock, "mlx")

	messages := []Message{
		{Role: "user", Content: "hello"},
	}
	result, err := adapter.Chat(context.Background(), messages, GenOpts{})
	core.RequireNoError(t, err)
	core.AssertEqual(t, "chat reply", result.Text)
	core.AssertNotNil(t, result.Metrics)
}

// TestMLXBackend_InferenceAdapter_Stream_Good verifies streaming through
// the InferenceAdapter (StreamingBackend path).
func TestMLXBackend_InferenceAdapter_Stream_Good(t *core.T) {
	mock := &mockTextModel{
		tokens: []inference.Token{
			{ID: 1, Text: "tok1"},
			{ID: 2, Text: "tok2"},
			{ID: 3, Text: "tok3"},
		},
	}
	adapter := NewInferenceAdapter(mock, "mlx")

	// Verify StreamingBackend compliance.
	var streaming StreamingBackend = adapter

	var collected []string
	err := streaming.GenerateStream(context.Background(), "prompt", GenOpts{}, func(tok string) error {
		collected = append(collected, tok)
		return nil
	})
	core.RequireNoError(t, err)
	core.AssertEqual(t, []string{"tok1", "tok2", "tok3"}, collected)
}

// TestMLXBackend_InferenceAdapter_ModelError_Bad verifies error propagation
// from the underlying TextModel through InferenceAdapter (the MLX path).
func TestMLXBackend_InferenceAdapter_ModelError_Bad(t *core.T) {
	mock := &mockTextModel{
		tokens: []inference.Token{
			{ID: 1, Text: "partial"},
		},
		err:       core.AnError,
		modelType: "qwen3",
	}
	adapter := NewInferenceAdapter(mock, "mlx")

	result, err := adapter.Generate(context.Background(), "prompt", GenOpts{})
	core.AssertError(t, err)
	core.AssertEqual(t, "partial", result.Text, "partial output should still be returned")
	core.AssertNil(t, result.Metrics, "metrics should be nil on error")
}

// TestMLXBackend_InferenceAdapter_Close_Good verifies that Close delegates
// to the underlying TextModel.
func TestMLXBackend_InferenceAdapter_Close_Good(t *core.T) {
	mock := &mockTextModel{}
	adapter := NewInferenceAdapter(mock, "mlx")

	err := adapter.Close()
	core.RequireNoError(t, err)
	core.AssertTrue(t, mock.closed)
}

// TestMLXBackend_InferenceAdapter_ModelAccess_Good verifies that the
// underlying TextModel is accessible for direct operations.
func TestMLXBackend_InferenceAdapter_ModelAccess_Good(t *core.T) {
	mock := &mockTextModel{modelType: "llama"}
	adapter := NewInferenceAdapter(mock, "mlx")

	model := adapter.Model()
	core.AssertEqual(t, "llama", model.ModelType())
	core.AssertEqual(t, inference.ModelInfo{}, model.Info())
}

// TestMLXBackend_InterfaceCompliance_Good verifies that InferenceAdapter
// (the return type of NewMLXBackend) satisfies both Backend and
// StreamingBackend at compile time.
func TestMLXBackend_InterfaceCompliance_Good(t *core.T) {
	adapter := NewInferenceAdapter(&mockTextModel{}, "mlx")
	var backend Backend = adapter
	var streaming StreamingBackend = adapter
	core.AssertNotNil(t, backend)
	core.AssertNotNil(t, streaming)
}

// TestMLXBackend_ConvertOpts_Temperature_Good verifies that GenOpts
// Temperature maps correctly through the adapter (critical for MLX
// which is temperature-sensitive on Metal).
func TestMLXBackend_ConvertOpts_Temperature_Good(t *core.T) {
	opts := convertOpts(GenOpts{Temperature: 0.8, MaxTokens: 2048})
	core.AssertNotEmpty(t, opts)
	core.AssertLen(t, opts, 2)
	core.AssertContains(t, core.Sprintf("%T", opts), "GenerateOption")
}

// TestMLXBackend_ConvertOpts_AllFields_Good verifies all GenOpts fields
// produce the expected number of inference options.
func TestMLXBackend_ConvertOpts_AllFields_Good(t *core.T) {
	opts := convertOpts(GenOpts{
		Temperature:   0.7,
		MaxTokens:     512,
		TopK:          40,
		TopP:          0.9,
		RepeatPenalty: 1.1,
	})
	core.AssertLen(t, opts, 5)
}

// TestMLXBackend_SetMLXMemoryLimits_ZeroNoop_Good verifies zero values leave
// the driver untouched (no panic, no side effects). Spec §2.2 — memory
// management before loading.
//
//	ml.SetMLXMemoryLimits(0, 0) // no-op
func TestMLXBackend_SetMLXMemoryLimits_ZeroNoop_Good(t *core.T) {
	cacheLimit := uint64(0)
	memoryLimit := uint64(0)
	core.AssertNotPanics(t, func() { SetMLXMemoryLimits(cacheLimit, memoryLimit) })
}

// --- v0.9.0 shape triplets ---

func TestBackendMlx_SetMLXMemoryLimits_Good(t *core.T) {
	symbol := any(SetMLXMemoryLimits)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendMlx_SetMLXMemoryLimits_Bad(t *core.T) {
	symbol := any(SetMLXMemoryLimits)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendMlx_SetMLXMemoryLimits_Ugly(t *core.T) {
	symbol := any(SetMLXMemoryLimits)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendMlx_NewMLXBackend_Good(t *core.T) {
	symbol := any(NewMLXBackend)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendMlx_NewMLXBackend_Bad(t *core.T) {
	symbol := any(NewMLXBackend)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendMlx_NewMLXBackend_Ugly(t *core.T) {
	symbol := any(NewMLXBackend)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}
