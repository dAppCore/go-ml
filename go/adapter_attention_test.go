// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"
	"dappco.re/go"

	"dappco.re/go/inference"
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

func TestInferenceAdapter_InspectAttention_Good(t *core.T) {
	adapter := NewInferenceAdapter(&mockAttentionModel{}, "test")
	snap, err := adapter.InspectAttention(context.Background(), "hello")
	core.RequireNoError(t, err)
	core.AssertEqual(t, 28, snap.NumLayers)
	core.AssertEqual(t, 8, snap.NumHeads)
	core.AssertEqual(t, 10, snap.SeqLen)
	core.AssertEqual(t, 64, snap.HeadDim)
	core.AssertEqual(t, "qwen3", snap.Architecture)
}

func TestInferenceAdapter_InspectAttention_Unsupported_Bad(t *core.T) {
	// Plain mockTextModel does not implement AttentionInspector.
	adapter := NewInferenceAdapter(&mockTextModel{}, "test")
	_, err := adapter.InspectAttention(context.Background(), "hello")
	core.AssertError(t, err)
	core.AssertContains(t, err.Error(), "does not support attention inspection")
}
