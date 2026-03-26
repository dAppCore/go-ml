package cmd

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"dappco.re/go/core"
	"dappco.re/go/core/ml"
	"forge.lthn.ai/core/cli/pkg/cli"
)

var serveCmd = &cli.Command{
	Use:   "serve",
	Short: "Start OpenAI-compatible inference server",
	Long:  "Starts an HTTP server serving /v1/completions and /v1/chat/completions using the configured ML backend.",
	RunE:  runServe,
}

var (
	serveBind        string
	serveModelPath   string
	serveThreads     int
	serveMaxTokens   int
	serveTimeout     int
	serveMaxRequests int
	serveMaxContext  int
)

func init() {
	serveCmd.Flags().StringVar(&serveBind, "bind", "0.0.0.0:8090", "Address to bind")
	serveCmd.Flags().StringVar(&serveModelPath, "model-path", "", "Path to model directory (for mlx backend)")
	serveCmd.Flags().IntVar(&serveThreads, "threads", 0, "Max CPU threads (0 = all available)")
	serveCmd.Flags().IntVar(&serveMaxTokens, "max-tokens", 4096, "Default max tokens per request")
	serveCmd.Flags().IntVar(&serveTimeout, "timeout", 300, "Request timeout in seconds")
	serveCmd.Flags().IntVar(&serveMaxRequests, "max-requests", 1, "Max concurrent requests (Metal is single-stream)")
	serveCmd.Flags().IntVar(&serveMaxContext, "max-context", 4, "Max chat messages to keep (sliding window, 0=unlimited)")
}

type completionRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	Stream      bool    `json:"stream"`
}

type completionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []completionChoice `json:"choices"`
	Usage   usageInfo          `json:"usage"`
}

type completionChoice struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
}

type chatRequest struct {
	Model       string       `json:"model"`
	Messages    []ml.Message `json:"messages"`
	MaxTokens   int          `json:"max_tokens"`
	Temperature float64      `json:"temperature"`
	Stream      bool         `json:"stream"`
}

type chatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message      ml.Message `json:"message"`
	Index        int        `json:"index"`
	FinishReason string     `json:"finish_reason"`
}

// SSE streaming types (OpenAI chunk format)
type chatChunkResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []chatChunkChoice `json:"choices"`
}

type chatChunkChoice struct {
	Delta        chatChunkDelta `json:"delta"`
	Index        int            `json:"index"`
	FinishReason *string        `json:"finish_reason"`
}

type chatChunkDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type completionChunkResponse struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []completionChunkChoice `json:"choices"`
}

type completionChunkChoice struct {
	Text         string  `json:"text"`
	Index        int     `json:"index"`
	FinishReason *string `json:"finish_reason"`
}

type usageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func runServe(cmd *cli.Command, args []string) error {
	// Cap CPU threads
	if serveThreads > 0 {
		prev := runtime.GOMAXPROCS(serveThreads)
		slog.Info("ml serve: capped threads", "threads", serveThreads, "previous", prev)
	}

	backend, err := createServeBackend()
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
			"max_tokens":      serveMaxTokens,
			"max_context":     serveMaxContext,
		})
	})

	mux.HandleFunc("POST /v1/completions", func(w http.ResponseWriter, r *http.Request) {
		// Concurrency gate
		if int(activeRequests.Load()) >= serveMaxRequests {
			http.Error(w, `{"error":"server busy, max concurrent requests reached"}`, http.StatusTooManyRequests)
			return
		}
		activeRequests.Add(1)
		defer activeRequests.Add(-1)

		// Request timeout
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(serveTimeout)*time.Second)
		defer cancel()
		r = r.WithContext(ctx)

		body, _ := io.ReadAll(r.Body)
		var req completionRequest
		if r := core.JSONUnmarshalString(string(body), &req); !r.OK {
			http.Error(w, r.Value.(error).Error(), 400)
			return
		}

		// Enforce server-level max-tokens cap
		if req.MaxTokens == 0 || req.MaxTokens > serveMaxTokens {
			req.MaxTokens = serveMaxTokens
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
				http.Error(w, "streaming not supported", 500)
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
				writeSSE(w, chunk)
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
			writeSSE(w, final)
			io.WriteString(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		// Non-streaming path
		res, err := backend.Generate(r.Context(), req.Prompt, opts)
		if err != nil {
			http.Error(w, err.Error(), 500)
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
	})

	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		// Concurrency gate
		if int(activeRequests.Load()) >= serveMaxRequests {
			http.Error(w, `{"error":"server busy, max concurrent requests reached"}`, http.StatusTooManyRequests)
			return
		}
		activeRequests.Add(1)
		defer activeRequests.Add(-1)

		// Request timeout
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(serveTimeout)*time.Second)
		defer cancel()
		r = r.WithContext(ctx)

		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		if r := core.JSONUnmarshalString(string(body), &req); !r.OK {
			http.Error(w, r.Value.(error).Error(), 400)
			return
		}

		// Enforce server-level max-tokens cap
		if req.MaxTokens == 0 || req.MaxTokens > serveMaxTokens {
			req.MaxTokens = serveMaxTokens
		}

		// Sliding window: keep system prompt (if any) + last N messages
		// Prevents KV-cache explosion on multi-turn conversations
		if serveMaxContext > 0 && len(req.Messages) > serveMaxContext {
			var kept []ml.Message
			rest := req.Messages
			// Preserve system message if present
			if len(rest) > 0 && rest[0].Role == "system" {
				kept = append(kept, rest[0])
				rest = rest[1:]
			}
			// Keep only the last N user/assistant messages
			if len(rest) > serveMaxContext {
				rest = rest[len(rest)-serveMaxContext:]
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
				http.Error(w, "streaming not supported", 500)
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
			writeSSE(w, roleChunk)
			flusher.Flush()

			err := streamer.ChatStream(r.Context(), req.Messages, opts, func(token string) error {
				chunk := chatChunkResponse{
					ID:      id,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   backend.Name(),
					Choices: []chatChunkChoice{{Delta: chatChunkDelta{Content: token}}},
				}
				writeSSE(w, chunk)
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
			writeSSE(w, final)
			io.WriteString(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		// Non-streaming path
		res, err := backend.Chat(r.Context(), req.Messages, opts)
		if err != nil {
			http.Error(w, err.Error(), 500)
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
	})

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
		io.WriteString(w, core.Sprintf(chatHTML, backend.Name(), serveMaxTokens))
	})

	slog.Info("ml serve: starting",
		"bind", serveBind,
		"backend", backend.Name(),
		"streaming", canStream,
		"threads", runtime.GOMAXPROCS(0),
		"max_tokens", serveMaxTokens,
		"max_context_msgs", serveMaxContext,
		"timeout_s", serveTimeout,
		"max_requests", serveMaxRequests,
	)
	core.Print(cmd.OutOrStdout(), "Serving on http://%s", serveBind)

	// Graceful shutdown on SIGINT/SIGTERM
	srv := &http.Server{
		Addr:    serveBind,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case <-shutdownCtx.Done():
		slog.Info("ml serve: shutting down", "reason", "signal")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("ml serve: shutdown error", "err", err)
			return err
		}
		slog.Info("ml serve: stopped cleanly")
		return nil
	case err := <-errCh:
		return err
	}
}

func writeJSON(w io.Writer, v any) {
	io.WriteString(w, core.JSONMarshalString(v))
}

func writeSSE(w io.Writer, v any) {
	io.WriteString(w, core.Sprintf("data: %s\n\n", core.JSONMarshalString(v)))
}
