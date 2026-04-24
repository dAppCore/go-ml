package cmd

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"dappco.re/go/core"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
)

// addServeCommand registers `ml serve` — starts an HTTP server serving
// /v1/completions and /v1/chat/completions using the configured ML backend.
//
//	core ml serve --bind 0.0.0.0:8090 --model-path ./model --max-tokens 4096
func addServeCommand(c *core.Core) {
	c.Command("ml/serve", core.Command{
		Description: "Start OpenAI-compatible inference server",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			bind := optStringOr(opts, "bind", "0.0.0.0:8090")
			modelPath := opts.String("model-path")
			threads := opts.Int("threads")
			maxTokens := optInt(opts, "max-tokens", 4096)
			timeoutSec := optInt(opts, "timeout", 300)
			maxRequests := optInt(opts, "max-requests", 1)
			maxContext := optInt(opts, "max-context", 4)

			return resultFromError(runServeLoop(bind, modelPath, threads, maxTokens, timeoutSec, maxRequests, maxContext))
		},
	})
}

// runServeLoop launches the HTTP server and blocks until a shutdown signal
// arrives or ListenAndServe returns.
//
//	err := runServeLoop(":8090", "", 0, 4096, 300, 1, 4)
func runServeLoop(bind, modelPath string, threads, maxTokens, timeoutSec, maxRequests, maxContext int) error {
	// Cap CPU threads
	if threads > 0 {
		prev := runtime.GOMAXPROCS(threads)
		slog.Info("ml serve: capped threads", "threads", threads, "previous", prev)
	}

	backend, err := createServeBackend(modelPath)
	if err != nil {
		return err
	}

	// Check if backend supports streaming
	streamer, canStream := backend.(ml.StreamingBackend)

	// Request tracking
	var activeRequests atomic.Int32
	startTime := time.Now()

	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]any{
			"status":          "ok",
			"model":           backend.Name(),
			"uptime_seconds":  int(time.Since(startTime).Seconds()),
			"active_requests": activeRequests.Load(),
			"max_threads":     runtime.GOMAXPROCS(0),
			"max_tokens":      maxTokens,
			"max_context":     maxContext,
		})
	})

	mux.HandleFunc("POST /v1/completions", handleCompletion(backend, streamer, canStream, &activeRequests, maxTokens, timeoutSec, maxRequests))
	mux.HandleFunc("POST /v1/chat/completions", handleChat(backend, streamer, canStream, &activeRequests, maxTokens, timeoutSec, maxRequests, maxContext))

	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Object string `json:"object"`
			Data   []struct {
				ID string `json:"id"`
			} `json:"data"`
		}{
			Object: "list",
			Data: []struct {
				ID string `json:"id"`
			}{{ID: backend.Name()}},
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, resp)
	})

	// Serve the lem-chat UI at root — same origin, no CORS needed
	mux.HandleFunc("GET /chat.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write(lemChatJS)
	})

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, core.Sprintf(chatHTML, backend.Name(), maxTokens))
	})

	slog.Info("ml serve: starting",
		"bind", bind,
		"backend", backend.Name(),
		"streaming", canStream,
		"threads", runtime.GOMAXPROCS(0),
		"max_tokens", maxTokens,
		"max_context_msgs", maxContext,
		"timeout_s", timeoutSec,
		"max_requests", maxRequests,
	)
	core.Print(nil, "Serving on http://%s", bind)

	// Graceful shutdown on SIGINT/SIGTERM
	srv := &http.Server{
		Addr:    bind,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		slog.Info("ml serve: shutting down", "reason", "signal")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("ml serve: shutdown error", "err", err)
			return coreerr.E("cmd.runServe", "shutdown", err)
		}
		slog.Info("ml serve: stopped cleanly")
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// handleCompletion returns an HTTP handler for /v1/completions.
//
//	mux.HandleFunc("POST /v1/completions", handleCompletion(backend, streamer, canStream, counter, 4096, 300, 1))
func handleCompletion(backend ml.Backend, streamer ml.StreamingBackend, canStream bool,
	activeRequests *atomic.Int32, maxTokens, timeoutSec, maxRequests int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Concurrency gate
		if int(activeRequests.Load()) >= maxRequests {
			http.Error(w, `{"error":"server busy, max concurrent requests reached"}`, http.StatusTooManyRequests)
			return
		}
		activeRequests.Add(1)
		defer activeRequests.Add(-1)

		// Request timeout
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutSec)*time.Second)
		defer cancel()
		r = r.WithContext(ctx)

		body, _ := io.ReadAll(r.Body)
		var req completionRequest
		if r := core.JSONUnmarshalString(string(body), &req); !r.OK {
			http.Error(w, r.Value.(error).Error(), http.StatusBadRequest)
			return
		}

		// Enforce server-level max-tokens cap
		if req.MaxTokens == 0 || req.MaxTokens > maxTokens {
			req.MaxTokens = maxTokens
		}

		opts := ml.GenOpts{
			Temperature: req.Temperature,
			MaxTokens:   req.MaxTokens,
			Model:       req.Model,
		}

		// Streaming path
		if req.Stream && canStream {
			id := core.Sprintf("cmpl-%d", time.Now().UnixNano())
			created := time.Now().Unix()

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming not supported", http.StatusInternalServerError)
				return
			}

			err := streamer.GenerateStream(r.Context(), req.Prompt, opts, func(token string) error {
				chunk := completionChunkResponse{
					ID:      id,
					Object:  "text_completion",
					Created: created,
					Model:   backend.Name(),
					Choices: []completionChunkChoice{{Text: token}},
				}
				io.WriteString(w, core.Sprintf("data: %s\n\n", core.JSONMarshalString(chunk)))
				flusher.Flush()
				return nil
			})

			if err != nil {
				slog.Error("stream error", "err", err)
			}

			// Send final chunk with finish_reason
			stop := "stop"
			final := completionChunkResponse{
				ID:      id,
				Object:  "text_completion",
				Created: created,
				Model:   backend.Name(),
				Choices: []completionChunkChoice{{FinishReason: &stop}},
			}
			io.WriteString(w, core.Sprintf("data: %s\n\n", core.JSONMarshalString(final)))
			io.WriteString(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		// Non-streaming path
		res, err := backend.Generate(r.Context(), req.Prompt, opts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := completionResponse{
			ID:      core.Sprintf("cmpl-%d", time.Now().UnixNano()),
			Object:  "text_completion",
			Created: time.Now().Unix(),
			Model:   backend.Name(),
			Choices: []completionChoice{{Text: res.Text, FinishReason: "stop"}},
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, resp)
	}
}

// handleChat returns an HTTP handler for /v1/chat/completions.
//
//	mux.HandleFunc("POST /v1/chat/completions", handleChat(backend, ...))
func handleChat(backend ml.Backend, streamer ml.StreamingBackend, canStream bool,
	activeRequests *atomic.Int32, maxTokens, timeoutSec, maxRequests, maxContext int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Concurrency gate
		if int(activeRequests.Load()) >= maxRequests {
			http.Error(w, `{"error":"server busy, max concurrent requests reached"}`, http.StatusTooManyRequests)
			return
		}
		activeRequests.Add(1)
		defer activeRequests.Add(-1)

		// Request timeout
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutSec)*time.Second)
		defer cancel()
		r = r.WithContext(ctx)

		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		if r := core.JSONUnmarshalString(string(body), &req); !r.OK {
			http.Error(w, r.Value.(error).Error(), http.StatusBadRequest)
			return
		}

		// Enforce server-level max-tokens cap
		if req.MaxTokens == 0 || req.MaxTokens > maxTokens {
			req.MaxTokens = maxTokens
		}

		// Sliding window: keep system prompt (if any) + last N messages
		// Prevents KV-cache explosion on multi-turn conversations
		if maxContext > 0 && len(req.Messages) > maxContext {
			var kept []ml.Message
			rest := req.Messages
			// Preserve system message if present
			if len(rest) > 0 && rest[0].Role == "system" {
				kept = append(kept, rest[0])
				rest = rest[1:]
			}
			// Keep only the last N user/assistant messages
			if len(rest) > maxContext {
				rest = rest[len(rest)-maxContext:]
			}
			req.Messages = append(kept, rest...)
			slog.Debug("ml serve: context window applied", "kept", len(req.Messages))
		}

		opts := ml.GenOpts{
			Temperature: req.Temperature,
			MaxTokens:   req.MaxTokens,
			Model:       req.Model,
		}

		// Streaming path
		if req.Stream && canStream {
			id := core.Sprintf("chatcmpl-%d", time.Now().UnixNano())
			created := time.Now().Unix()

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming not supported", http.StatusInternalServerError)
				return
			}

			// Send initial role chunk
			roleChunk := chatChunkResponse{
				ID:      id,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   backend.Name(),
				Choices: []chatChunkChoice{{Delta: chatChunkDelta{Role: "assistant"}}},
			}
			io.WriteString(w, core.Sprintf("data: %s\n\n", core.JSONMarshalString(roleChunk)))
			flusher.Flush()

			err := streamer.ChatStream(r.Context(), req.Messages, opts, func(token string) error {
				chunk := chatChunkResponse{
					ID:      id,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   backend.Name(),
					Choices: []chatChunkChoice{{Delta: chatChunkDelta{Content: token}}},
				}
				io.WriteString(w, core.Sprintf("data: %s\n\n", core.JSONMarshalString(chunk)))
				flusher.Flush()
				return nil
			})

			if err != nil {
				slog.Error("stream error", "err", err)
			}

			// Send final chunk with finish_reason
			stop := "stop"
			final := chatChunkResponse{
				ID:      id,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   backend.Name(),
				Choices: []chatChunkChoice{{Delta: chatChunkDelta{}, FinishReason: &stop}},
			}
			io.WriteString(w, core.Sprintf("data: %s\n\n", core.JSONMarshalString(final)))
			io.WriteString(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		// Non-streaming path
		res, err := backend.Chat(r.Context(), req.Messages, opts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := chatResponse{
			ID:      core.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   backend.Name(),
			Choices: []chatChoice{{
				Message:      ml.Message{Role: "assistant", Content: res.Text},
				FinishReason: "stop",
			}},
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, resp)
	}
}

// completionRequest is the OpenAI-compatible /v1/completions request body.
type completionRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	Stream      bool    `json:"stream"`
}

// completionResponse is the OpenAI-compatible /v1/completions response body.
type completionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []completionChoice `json:"choices"`
	Usage   usageInfo          `json:"usage"`
}

// completionChoice is one completion choice in a completionResponse.
type completionChoice struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
}

// chatRequest is the OpenAI-compatible /v1/chat/completions request body.
type chatRequest struct {
	Model       string       `json:"model"`
	Messages    []ml.Message `json:"messages"`
	MaxTokens   int          `json:"max_tokens"`
	Temperature float64      `json:"temperature"`
	Stream      bool         `json:"stream"`
}

// chatResponse is the OpenAI-compatible /v1/chat/completions response body.
type chatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
}

// chatChoice is one choice in a chatResponse.
type chatChoice struct {
	Message      ml.Message `json:"message"`
	Index        int        `json:"index"`
	FinishReason string     `json:"finish_reason"`
}

// chatChunkResponse is the SSE streaming chunk for /v1/chat/completions.
type chatChunkResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []chatChunkChoice `json:"choices"`
}

// chatChunkChoice is one streaming choice.
type chatChunkChoice struct {
	Delta        chatChunkDelta `json:"delta"`
	Index        int            `json:"index"`
	FinishReason *string        `json:"finish_reason"`
}

// chatChunkDelta is the content fragment in a chat streaming chunk.
type chatChunkDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// completionChunkResponse is the SSE streaming chunk for /v1/completions.
type completionChunkResponse struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []completionChunkChoice `json:"choices"`
}

// completionChunkChoice is one streaming completion choice.
type completionChunkChoice struct {
	Text         string  `json:"text"`
	Index        int     `json:"index"`
	FinishReason *string `json:"finish_reason"`
}

// usageInfo is the OpenAI token usage summary.
type usageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// writeJSON writes a JSON-marshalled object to the response writer.
//
//	writeJSON(w, map[string]any{"status": "ok"})
func writeJSON(w io.Writer, v any) {
	io.WriteString(w, core.JSONMarshalString(v))
}
