// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"
	"testing"

	"dappco.re/go/core/inference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAttentionModel extends mockTextModel with AttentionInspector support.
type mockAttentionModel struct {
	mockTextModel
}

func (m *mockAttentionModel) InspectAttention(_ context.Context, _ string, _ ...inference.GenerateOption) (*inference.AttentionSnapshot, error) {
	return &inference.AttentionSnapshot{
		NumLayers:    28,
		NumHeads:     8,
		SeqLen:       10,
		HeadDim:      64,
		Architecture: "qwen3",
	}, nil
}

func TestInferenceAdapter_InspectAttention_Good(t *testing.T) {
	adapter := NewInferenceAdapter(&mockAttentionModel{}, "test")
	snap, err := adapter.InspectAttention(context.Background(), "hello")
	require.NoError(t, err)
	assert.Equal(t, 28, snap.NumLayers)
	assert.Equal(t, 8, snap.NumHeads)
	assert.Equal(t, 10, snap.SeqLen)
	assert.Equal(t, 64, snap.HeadDim)
	assert.Equal(t, "qwen3", snap.Architecture)
}

func TestInferenceAdapter_InspectAttention_Unsupported_Bad(t *testing.T) {
	// Plain mockTextModel does not implement AttentionInspector.
	adapter := NewInferenceAdapter(&mockTextModel{}, "test")
	_, err := adapter.InspectAttention(context.Background(), "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support attention inspection")
}
