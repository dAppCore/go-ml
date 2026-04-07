// SPDX-Licence-Identifier: EUPL-1.2

//go:build darwin && arm64 && !nomlx

package ml

import (
	"context"
	"testing"

	"dappco.re/go/core/inference"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// backend_mlx.go tests — uses mockTextModel from adapter_test.go
// since we cannot load real MLX models in CI
// ---------------------------------------------------------------------------

// TestMLXBackend_InferenceAdapter_Generate_Good verifies that an
// InferenceAdapter (the type returned by NewMLXBackend) correctly
// generates text through a mock TextModel.
func TestMLXBackend_InferenceAdapter_Generate_Good(t *testing.T) {
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
	assert.Equal(t, "mlx", backend.Name())
	assert.True(t, backend.Available())

	result, err := backend.Generate(context.Background(), "prompt", GenOpts{Temperature: 0.5})
	require.NoError(t, err)
	assert.Equal(t, "MLX output", result.Text)
	assert.NotNil(t, result.Metrics)
}

// TestMLXBackend_InferenceAdapter_Chat_Good verifies chat through the
// InferenceAdapter wrapper (the path NewMLXBackend takes).
func TestMLXBackend_InferenceAdapter_Chat_Good(t *testing.T) {
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
	require.NoError(t, err)
	assert.Equal(t, "chat reply", result.Text)
	assert.NotNil(t, result.Metrics)
}

// TestMLXBackend_InferenceAdapter_Stream_Good verifies streaming through
// the InferenceAdapter (StreamingBackend path).
func TestMLXBackend_InferenceAdapter_Stream_Good(t *testing.T) {
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
	require.NoError(t, err)
	assert.Equal(t, []string{"tok1", "tok2", "tok3"}, collected)
}

// TestMLXBackend_InferenceAdapter_ModelError_Bad verifies error propagation
// from the underlying TextModel through InferenceAdapter (the MLX path).
func TestMLXBackend_InferenceAdapter_ModelError_Bad(t *testing.T) {
	mock := &mockTextModel{
		tokens: []inference.Token{
			{ID: 1, Text: "partial"},
		},
		err:       assert.AnError,
		modelType: "qwen3",
	}
	adapter := NewInferenceAdapter(mock, "mlx")

	result, err := adapter.Generate(context.Background(), "prompt", GenOpts{})
	assert.Error(t, err)
	assert.Equal(t, "partial", result.Text, "partial output should still be returned")
	assert.Nil(t, result.Metrics, "metrics should be nil on error")
}

// TestMLXBackend_InferenceAdapter_Close_Good verifies that Close delegates
// to the underlying TextModel.
func TestMLXBackend_InferenceAdapter_Close_Good(t *testing.T) {
	mock := &mockTextModel{}
	adapter := NewInferenceAdapter(mock, "mlx")

	err := adapter.Close()
	require.NoError(t, err)
	assert.True(t, mock.closed)
}

// TestMLXBackend_InferenceAdapter_ModelAccess_Good verifies that the
// underlying TextModel is accessible for direct operations.
func TestMLXBackend_InferenceAdapter_ModelAccess_Good(t *testing.T) {
	mock := &mockTextModel{modelType: "llama"}
	adapter := NewInferenceAdapter(mock, "mlx")

	model := adapter.Model()
	assert.Equal(t, "llama", model.ModelType())
	assert.Equal(t, inference.ModelInfo{}, model.Info())
}

// TestMLXBackend_InterfaceCompliance_Good verifies that InferenceAdapter
// (the return type of NewMLXBackend) satisfies both Backend and
// StreamingBackend at compile time.
func TestMLXBackend_InterfaceCompliance_Good(t *testing.T) {
	var _ Backend = (*InferenceAdapter)(nil)
	var _ StreamingBackend = (*InferenceAdapter)(nil)
}

// TestMLXBackend_ConvertOpts_Temperature_Good verifies that GenOpts
// Temperature maps correctly through the adapter (critical for MLX
// which is temperature-sensitive on Metal).
func TestMLXBackend_ConvertOpts_Temperature_Good(t *testing.T) {
	opts := convertOpts(GenOpts{Temperature: 0.8, MaxTokens: 2048})
	assert.Len(t, opts, 2)
}

// TestMLXBackend_ConvertOpts_AllFields_Good verifies all GenOpts fields
// produce the expected number of inference options.
func TestMLXBackend_ConvertOpts_AllFields_Good(t *testing.T) {
	opts := convertOpts(GenOpts{
		Temperature:   0.7,
		MaxTokens:     512,
		TopK:          40,
		TopP:          0.9,
		RepeatPenalty: 1.1,
	})
	assert.Len(t, opts, 5)
}
