package ml

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"dappco.re/go/core"
	"dappco.re/go/inference"
	coreio "dappco.re/go/io"
	coreerr "dappco.re/go/log"
)

// Compile-time check: HTTPBackend satisfies inference.Backend (spec §2.1).
var _ inference.Backend = (*HTTPBackend)(nil)

// HTTPBackend talks to an OpenAI-compatible chat completions API.
type HTTPBackend struct {
	baseURL    string
	model      string
	maxTokens  int
	httpClient *http.Client
	medium     coreio.Medium
}

// HTTPOption configures an HTTPBackend at construction time.
//
//	b := ml.NewHTTPBackend("http://localhost:11434", "llama3",
//	    ml.WithHTTPClient(myClient),
//	    ml.WithMedium(io.S3("models.lthn.io")),
//	)
type HTTPOption func(*HTTPBackend)

// WithHTTPClient overrides the default net/http.Client used for requests.
func WithHTTPClient(client *http.Client) HTTPOption {
	return func(b *HTTPBackend) {
		if client != nil {
			b.httpClient = client
		}
	}
}

// WithMedium attaches an io.Medium so model artefacts (LoRA adapters,
// GGUF blobs, streamed responses) can be loaded or staged from any
// supported backend (local disk, S3, in-memory, etc.).
//
//	b := ml.NewHTTPBackend(url, model, ml.WithMedium(io.S3("models.lthn.io")))
func WithMedium(medium coreio.Medium) HTTPOption {
	return func(b *HTTPBackend) {
		b.medium = medium
	}
}

// WithHTTPMaxTokens sets the default maximum token count for requests.
func WithHTTPMaxTokens(n int) HTTPOption {
	return func(b *HTTPBackend) {
		b.maxTokens = n
	}
}

// chatRequest is the request body for /v1/chat/completions.
type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// chatChoice is a single completion choice.
type chatChoice struct {
	Message Message `json:"message"`
}

// chatResponse is the response from /v1/chat/completions.
type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

// retryableError marks errors that should be retried.
type retryableError struct {
	err error
}

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

// NewHTTPBackend creates an HTTPBackend for the given base URL and model.
// Additional options configure the HTTP client, default max tokens, or an
// io.Medium used for staging model artefacts.
//
//	b := ml.NewHTTPBackend("http://localhost:11434", "llama3")
//	b := ml.NewHTTPBackend(url, model, ml.WithMedium(io.S3("models.lthn.io")))
func NewHTTPBackend(baseURL, model string, opts ...HTTPOption) *HTTPBackend {
	b := &HTTPBackend{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Medium returns the io.Medium configured via WithMedium, or nil if none
// was supplied.
func (b *HTTPBackend) Medium() coreio.Medium { return b.medium }

// Name returns "http".
func (b *HTTPBackend) Name() string { return "http" }

// Available always returns true for HTTP backends.
func (b *HTTPBackend) Available() bool { return b.baseURL != "" }

// Model returns the configured model name.
func (b *HTTPBackend) Model() string { return b.model }

// BaseURL returns the configured base URL.
func (b *HTTPBackend) BaseURL() string { return b.baseURL }

// SetMaxTokens sets the maximum token count for requests.
func (b *HTTPBackend) SetMaxTokens(n int) { b.maxTokens = n }

// LoadModel satisfies inference.Backend by wrapping the HTTPBackend as an
// inference.TextModel. The path argument is ignored — HTTP backends talk to
// a remote server which already has the model loaded. Spec §2.3.
//
//	backend := ml.NewHTTPBackend("http://localhost:11434", "llama2")
//	model, _ := backend.LoadModel("dummy")
//	for tok := range model.Generate(ctx, "hello") {
//	    fmt.Print(tok.Text)
//	}
func (b *HTTPBackend) LoadModel(_ string, _ ...inference.LoadOption) (inference.TextModel, error) {
	return NewHTTPTextModel(b), nil
}

// Generate sends a single prompt and returns the response.
func (b *HTTPBackend) Generate(ctx context.Context, prompt string, opts GenOpts) (Result, error) {
	return b.Chat(ctx, []Message{{Role: "user", Content: prompt}}, opts)
}

// Chat sends a multi-turn conversation and returns the response.
// Retries up to 3 times with exponential backoff on transient failures.
func (b *HTTPBackend) Chat(ctx context.Context, messages []Message, opts GenOpts) (Result, error) {
	model := b.model
	if opts.Model != "" {
		model = opts.Model
	}
	maxTokens := b.maxTokens
	if opts.MaxTokens > 0 {
		maxTokens = opts.MaxTokens
	}
	temp := opts.Temperature

	req := chatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temp,
		MaxTokens:   maxTokens,
	}

	body := []byte(core.JSONMarshalString(req))

	const maxAttempts = 3
	var lastErr error

	for attempt := range maxAttempts {
		if attempt > 0 {
			backoff := time.Duration(100<<uint(attempt-1)) * time.Millisecond
			time.Sleep(backoff)
		}

		result, err := b.doRequest(ctx, body)
		if err == nil {
			return newResult(applyStopSequences(result, opts.StopSequences), nil), nil
		}
		lastErr = err

		var re *retryableError
		if !core.As(err, &re) {
			return Result{}, err
		}
	}

	return Result{}, coreerr.E("ml.HTTPBackend.Chat", core.Sprintf("exhausted %d retries", maxAttempts), lastErr)
}

// doRequest sends a single HTTP request and parses the response.
func (b *HTTPBackend) doRequest(ctx context.Context, body []byte) (string, error) {
	url := b.baseURL + "/v1/chat/completions"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", coreerr.E("ml.HTTPBackend.doRequest", "create request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return "", &retryableError{coreerr.E("ml.HTTPBackend.doRequest", "http request", err)}
	}
	defer resp.Body.Close()

	respBody, err := readAll(resp.Body)
	if err != nil {
		return "", &retryableError{coreerr.E("ml.HTTPBackend.doRequest", "read response", err)}
	}

	if resp.StatusCode >= 500 {
		return "", &retryableError{coreerr.E("ml.HTTPBackend.doRequest", core.Sprintf("server error %d: %s", resp.StatusCode, string(respBody)), nil)}
	}
	if resp.StatusCode != http.StatusOK {
		return "", coreerr.E("ml.HTTPBackend.doRequest", core.Sprintf("unexpected status %d: %s", resp.StatusCode, string(respBody)), nil)
	}

	var chatResp chatResponse
	if r := core.JSONUnmarshal(respBody, &chatResp); !r.OK {
		return "", coreerr.E("ml.HTTPBackend.doRequest", "unmarshal response", r.Value.(error))
	}

	if len(chatResp.Choices) == 0 {
		return "", coreerr.E("ml.HTTPBackend.doRequest", "no choices in response", nil)
	}

	return chatResp.Choices[0].Message.Content, nil
}
