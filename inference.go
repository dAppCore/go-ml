// Package ml provides ML inference, scoring, and model management for CoreGo.
//
// It supports multiple inference backends (HTTP, llama-server, Ollama) through
// a common Backend interface, and includes an ethics-aware scoring engine with
// both heuristic and LLM-judge capabilities.
//
// Register as a CoreGo service:
//
//	core.New(
//	    core.WithService(ml.NewService),
//	)
package ml

import (
	"context"

	"dappco.re/go/core/inference"
)

// Result holds the response text and optional inference metrics.
// Backends that support metrics (e.g. MLX via InferenceAdapter) populate
// Metrics; HTTP and subprocess backends leave it nil.
type Result struct {
	Text    string
	Metrics *inference.GenerateMetrics
}

// Backend generates text from prompts. Implementations include HTTPBackend
// (OpenAI-compatible API), LlamaBackend (managed llama-server process), and
// OllamaBackend (Ollama native API).
type Backend interface {
	// Generate sends a single user prompt and returns the response.
	Generate(ctx context.Context, prompt string, opts GenOpts) (Result, error)

	// Chat sends a multi-turn conversation and returns the response.
	Chat(ctx context.Context, messages []Message, opts GenOpts) (Result, error)

	// Name returns the backend identifier (e.g. "http", "llama", "ollama").
	Name() string

	// Available reports whether the backend is ready to accept requests.
	Available() bool
}

// GenOpts configures a generation request.
type GenOpts struct {
	Temperature   float64
	MaxTokens     int
	Model         string   // override model for this request
	TopK          int      // top-k sampling (0 = disabled)
	TopP          float64  // nucleus sampling threshold (0 = disabled)
	RepeatPenalty float64  // repetition penalty (0 = disabled, 1.0 = no penalty)
	StopTokens    []int32  // token IDs that terminate generation early
	StopSequences []string // literal substrings that terminate generation early
}

// Message is a type alias for inference.Message, providing backward compatibility.
// All callers continue using ml.Message — it is the same underlying type.
type Message = inference.Message

// TokenCallback receives each generated token as text. Return a non-nil
// error to stop generation early (e.g. client disconnect).
type TokenCallback func(token string) error

// Deprecated: StreamingBackend is retained for backward compatibility.
// New code should use inference.TextModel with iter.Seq[Token] directly.
// See InferenceAdapter for the bridge pattern.
type StreamingBackend interface {
	Backend

	// GenerateStream streams tokens from a single prompt via the callback.
	GenerateStream(ctx context.Context, prompt string, opts GenOpts, cb TokenCallback) error

	// ChatStream streams tokens from a chat conversation via the callback.
	ChatStream(ctx context.Context, messages []Message, opts GenOpts, cb TokenCallback) error
}

// DefaultGenOpts returns sensible defaults for generation.
func DefaultGenOpts() GenOpts {
	return GenOpts{
		Temperature: 0.1,
	}
}
