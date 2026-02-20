// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"
	"strings"

	"forge.lthn.ai/core/go-inference"
)

// InferenceAdapter bridges a go-inference TextModel (iter.Seq[Token]) to the
// ml.Backend and ml.StreamingBackend interfaces (string returns / TokenCallback).
//
// This is the key adapter for Phase 1: any go-inference backend (MLX Metal,
// ROCm, llama.cpp) can be wrapped to satisfy go-ml's Backend contract.
type InferenceAdapter struct {
	model inference.TextModel
	name  string
}

// Compile-time checks.
var _ Backend = (*InferenceAdapter)(nil)
var _ StreamingBackend = (*InferenceAdapter)(nil)

// NewInferenceAdapter wraps a go-inference TextModel as an ml.Backend and
// ml.StreamingBackend. The name is used for Backend.Name() (e.g. "mlx").
func NewInferenceAdapter(model inference.TextModel, name string) *InferenceAdapter {
	return &InferenceAdapter{model: model, name: name}
}

// Generate collects all tokens from the model's iterator into a single string.
func (a *InferenceAdapter) Generate(ctx context.Context, prompt string, opts GenOpts) (string, error) {
	inferOpts := convertOpts(opts)
	var b strings.Builder
	for tok := range a.model.Generate(ctx, prompt, inferOpts...) {
		b.WriteString(tok.Text)
	}
	if err := a.model.Err(); err != nil {
		return b.String(), err
	}
	return b.String(), nil
}

// Chat converts ml.Message to inference.Message, then collects all tokens.
func (a *InferenceAdapter) Chat(ctx context.Context, messages []Message, opts GenOpts) (string, error) {
	inferMsgs := convertMessages(messages)
	inferOpts := convertOpts(opts)
	var b strings.Builder
	for tok := range a.model.Chat(ctx, inferMsgs, inferOpts...) {
		b.WriteString(tok.Text)
	}
	if err := a.model.Err(); err != nil {
		return b.String(), err
	}
	return b.String(), nil
}

// GenerateStream forwards each generated token's text to the callback.
// Returns nil on success, the callback's error if it stops early, or the
// model's error if generation fails.
func (a *InferenceAdapter) GenerateStream(ctx context.Context, prompt string, opts GenOpts, cb TokenCallback) error {
	inferOpts := convertOpts(opts)
	for tok := range a.model.Generate(ctx, prompt, inferOpts...) {
		if err := cb(tok.Text); err != nil {
			return err
		}
	}
	return a.model.Err()
}

// ChatStream forwards each generated chat token's text to the callback.
func (a *InferenceAdapter) ChatStream(ctx context.Context, messages []Message, opts GenOpts, cb TokenCallback) error {
	inferMsgs := convertMessages(messages)
	inferOpts := convertOpts(opts)
	for tok := range a.model.Chat(ctx, inferMsgs, inferOpts...) {
		if err := cb(tok.Text); err != nil {
			return err
		}
	}
	return a.model.Err()
}

// Name returns the backend identifier set at construction.
func (a *InferenceAdapter) Name() string { return a.name }

// Available always returns true — the model is already loaded.
func (a *InferenceAdapter) Available() bool { return true }

// Close delegates to the underlying TextModel.Close(), releasing GPU memory
// and other resources.
func (a *InferenceAdapter) Close() error { return a.model.Close() }

// Model returns the underlying go-inference TextModel for direct access
// to Classify, BatchGenerate, Metrics, Info, etc.
func (a *InferenceAdapter) Model() inference.TextModel { return a.model }

// convertOpts maps ml.GenOpts to go-inference functional options.
func convertOpts(opts GenOpts) []inference.GenerateOption {
	var out []inference.GenerateOption
	if opts.Temperature != 0 {
		out = append(out, inference.WithTemperature(float32(opts.Temperature)))
	}
	if opts.MaxTokens != 0 {
		out = append(out, inference.WithMaxTokens(opts.MaxTokens))
	}
	// GenOpts.Model is ignored — the model is already loaded.
	return out
}

// convertMessages maps ml.Message to inference.Message (trivial field copy).
func convertMessages(msgs []Message) []inference.Message {
	out := make([]inference.Message, len(msgs))
	for i, m := range msgs {
		out[i] = inference.Message{Role: m.Role, Content: m.Content}
	}
	return out
}
