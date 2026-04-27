// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"
	"iter"
	"testing"

	"dappco.re/go/core"
	"dappco.re/go/inference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTextModel implements inference.TextModel for testing the InferenceAdapter.
type mockTextModel struct {
	tokens    []inference.Token // tokens to yield
	err       error             // error to return from Err()
	closed    bool
	modelType string
}

func (m *mockTextModel) Generate(_ context.Context, _ string, _ ...inference.GenerateOption) iter.Seq[inference.Token] {
	return func(yield func(inference.Token) bool) {
		for _, tok := range m.tokens {
			if !yield(tok) {
				return
			}
		}
	}
}

func (m *mockTextModel) Chat(_ context.Context, _ []inference.Message, _ ...inference.GenerateOption) iter.Seq[inference.Token] {
	return func(yield func(inference.Token) bool) {
		for _, tok := range m.tokens {
			if !yield(tok) {
				return
			}
		}
	}
}

func (m *mockTextModel) Classify(_ context.Context, _ []string, _ ...inference.GenerateOption) ([]inference.ClassifyResult, error) {
	panic("Classify not used by adapter")
}

func (m *mockTextModel) BatchGenerate(_ context.Context, _ []string, _ ...inference.GenerateOption) ([]inference.BatchResult, error) {
	panic("BatchGenerate not used by adapter")
}

func (m *mockTextModel) ModelType() string                  { return m.modelType }
func (m *mockTextModel) Info() inference.ModelInfo          { return inference.ModelInfo{} }
func (m *mockTextModel) Metrics() inference.GenerateMetrics { return inference.GenerateMetrics{} }
func (m *mockTextModel) Err() error                         { return m.err }
func (m *mockTextModel) Close() error                       { m.closed = true; return nil }

// --- Tests ---

func TestInferenceAdapter_Generate_Good(t *testing.T) {
	mock := &mockTextModel{
		tokens: []inference.Token{
			{ID: 1, Text: "Hello"},
			{ID: 2, Text: " "},
			{ID: 3, Text: "world"},
		},
	}
	adapter := NewInferenceAdapter(mock, "test")

	result, err := adapter.Generate(context.Background(), "prompt", GenOpts{})
	require.NoError(t, err)
	assert.Equal(t, "Hello world", result.Text)
	assert.NotNil(t, result.Metrics)
}

func TestInferenceAdapter_Generate_Empty_Good(t *testing.T) {
	mock := &mockTextModel{tokens: nil}
	adapter := NewInferenceAdapter(mock, "test")

	result, err := adapter.Generate(context.Background(), "prompt", GenOpts{})
	require.NoError(t, err)
	assert.Equal(t, "", result.Text)
	assert.NotNil(t, result.Metrics)
}

func TestInferenceAdapter_Generate_ModelError_Bad(t *testing.T) {
	mock := &mockTextModel{
		tokens: []inference.Token{
			{ID: 1, Text: "partial"},
		},
		err: core.NewError("out of memory"),
	}
	adapter := NewInferenceAdapter(mock, "test")

	result, err := adapter.Generate(context.Background(), "prompt", GenOpts{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of memory")
	// Partial output is still returned.
	assert.Equal(t, "partial", result.Text)
	// Metrics are nil on error.
	assert.Nil(t, result.Metrics)
}

func TestInferenceAdapter_GenerateStream_Good(t *testing.T) {
	mock := &mockTextModel{
		tokens: []inference.Token{
			{ID: 1, Text: "one"},
			{ID: 2, Text: "two"},
			{ID: 3, Text: "three"},
		},
	}
	adapter := NewInferenceAdapter(mock, "test")

	var collected []string
	err := adapter.GenerateStream(context.Background(), "prompt", GenOpts{}, func(token string) error {
		collected = append(collected, token)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"one", "two", "three"}, collected)
}

func TestInferenceAdapter_GenerateStream_CallbackError_Bad(t *testing.T) {
	callbackErr := core.NewError("client disconnected")
	mock := &mockTextModel{
		tokens: []inference.Token{
			{ID: 1, Text: "one"},
			{ID: 2, Text: "two"},
			{ID: 3, Text: "three"},
		},
	}
	adapter := NewInferenceAdapter(mock, "test")

	count := 0
	err := adapter.GenerateStream(context.Background(), "prompt", GenOpts{}, func(token string) error {
		count++
		if count >= 2 {
			return callbackErr
		}
		return nil
	})
	assert.ErrorIs(t, err, callbackErr)
	assert.Equal(t, 2, count, "callback should have been called exactly twice")
}

func TestInferenceAdapter_ContextCancellation_Bad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a mock that respects context cancellation.
	mock := &mockTextModel{}
	mock.tokens = nil // no tokens; the mock Generate just returns empty
	// Simulate context cancel causing model error.
	cancel()
	mock.err = ctx.Err()

	adapter := NewInferenceAdapter(mock, "test")
	_, err := adapter.Generate(ctx, "prompt", GenOpts{})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestInferenceAdapter_Chat_Good(t *testing.T) {
	mock := &mockTextModel{
		tokens: []inference.Token{
			{ID: 1, Text: "Hi"},
			{ID: 2, Text: " there"},
		},
	}
	adapter := NewInferenceAdapter(mock, "test")

	messages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
		{Role: "user", Content: "How are you?"},
	}
	result, err := adapter.Chat(context.Background(), messages, GenOpts{})
	require.NoError(t, err)
	assert.Equal(t, "Hi there", result.Text)
	assert.NotNil(t, result.Metrics)
}

func TestInferenceAdapter_ChatStream_Good(t *testing.T) {
	mock := &mockTextModel{
		tokens: []inference.Token{
			{ID: 1, Text: "reply"},
			{ID: 2, Text: "!"},
		},
	}
	adapter := NewInferenceAdapter(mock, "test")

	messages := []Message{{Role: "user", Content: "test"}}
	var collected []string
	err := adapter.ChatStream(context.Background(), messages, GenOpts{}, func(token string) error {
		collected = append(collected, token)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"reply", "!"}, collected)
}

func TestInferenceAdapter_StopSequences_Good(t *testing.T) {
	mock := &mockTextModel{
		tokens: []inference.Token{
			{ID: 1, Text: "hello "},
			{ID: 2, Text: "STOP world"},
			{ID: 3, Text: "ignored"},
		},
	}
	adapter := NewInferenceAdapter(mock, "test")

	result, err := adapter.Generate(context.Background(), "prompt", GenOpts{StopSequences: []string{"STOP"}})
	require.NoError(t, err)
	assert.Equal(t, "hello ", result.Text)

	var collected []string
	err = adapter.GenerateStream(context.Background(), "prompt", GenOpts{StopSequences: []string{"STOP"}}, func(token string) error {
		collected = append(collected, token)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"hello "}, collected)
}

func TestInferenceAdapter_ConvertOpts_Good(t *testing.T) {
	// Non-zero values should produce options.
	opts := convertOpts(GenOpts{Temperature: 0.7, MaxTokens: 512, Model: "ignored"})
	assert.Len(t, opts, 2)

	// Zero values should produce no options.
	opts = convertOpts(GenOpts{})
	assert.Len(t, opts, 0)

	// Only temperature set.
	opts = convertOpts(GenOpts{Temperature: 0.5})
	assert.Len(t, opts, 1)

	// Only max tokens set.
	opts = convertOpts(GenOpts{MaxTokens: 100})
	assert.Len(t, opts, 1)
}

func TestInferenceAdapter_ConvertOpts_NewFields_Good(t *testing.T) {
	// TopK only.
	opts := convertOpts(GenOpts{TopK: 40})
	assert.Len(t, opts, 1)

	// TopP only.
	opts = convertOpts(GenOpts{TopP: 0.9})
	assert.Len(t, opts, 1)

	// RepeatPenalty only.
	opts = convertOpts(GenOpts{RepeatPenalty: 1.1})
	assert.Len(t, opts, 1)

	// All new fields set together.
	opts = convertOpts(GenOpts{TopK: 40, TopP: 0.9, RepeatPenalty: 1.1})
	assert.Len(t, opts, 3)

	// All fields set (Temperature + MaxTokens + TopK + TopP + RepeatPenalty).
	opts = convertOpts(GenOpts{
		Temperature:   0.7,
		MaxTokens:     512,
		TopK:          40,
		TopP:          0.9,
		RepeatPenalty: 1.1,
	})
	assert.Len(t, opts, 5)

	// Zero TopK/TopP/RepeatPenalty should not produce options.
	opts = convertOpts(GenOpts{Temperature: 0.5, TopK: 0, TopP: 0, RepeatPenalty: 0})
	assert.Len(t, opts, 1) // only Temperature
}

func TestInferenceAdapter_MessageAlias_Good(t *testing.T) {
	// ml.Message and inference.Message are the same type — verify interchangeability.
	mlMsg := Message{Role: "user", Content: "Hello"}
	inferMsg := inference.Message{Role: "user", Content: "Hello"}
	assert.Equal(t, mlMsg, inferMsg)

	// Can assign directly without conversion.
	var msgs []inference.Message
	msgs = append(msgs, mlMsg)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "Hello", msgs[0].Content)
}

func TestInferenceAdapter_NameAndAvailable_Good(t *testing.T) {
	mock := &mockTextModel{}
	adapter := NewInferenceAdapter(mock, "mlx")

	assert.Equal(t, "mlx", adapter.Name())
	assert.True(t, adapter.Available())
}

func TestInferenceAdapter_Close_Good(t *testing.T) {
	mock := &mockTextModel{}
	adapter := NewInferenceAdapter(mock, "test")

	err := adapter.Close()
	require.NoError(t, err)
	assert.True(t, mock.closed)
}

func TestInferenceAdapter_Model_Good(t *testing.T) {
	mock := &mockTextModel{modelType: "qwen3"}
	adapter := NewInferenceAdapter(mock, "test")

	assert.Equal(t, "qwen3", adapter.Model().ModelType())
}
