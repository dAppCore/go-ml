// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// LlamaBackend unit tests — no subprocess, HTTP mocked via httptest
// ---------------------------------------------------------------------------

// newMockLlamaServer creates an httptest.Server that responds to both
// /health and /v1/chat/completions.  Returns a fixed content string for chat
// and 200 OK for health.
func newMockLlamaServer(t *testing.T, chatContent string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/chat/completions":
			resp := chatResponse{
				Choices: []chatChoice{
					{Message: Message{Role: "assistant", Content: chatContent}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("encode mock response: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
}

// newLlamaBackendWithServer wires up a LlamaBackend pointing at the given
// test server.  The procID is set so Available() attempts the health check.
func newLlamaBackendWithServer(srv *httptest.Server) *LlamaBackend {
	return &LlamaBackend{
		procID: "test-proc",
		port:   serverPort(srv),
		http:   NewHTTPBackend(srv.URL, ""),
	}
}

// serverPort extracts the port number from an httptest.Server.
func serverPort(srv *httptest.Server) int {
	u, _ := url.Parse(srv.URL)
	p, _ := strconv.Atoi(u.Port())
	return p
}

// --- Name ---

func TestLlamaBackend_Name_Good(t *testing.T) {
	lb := &LlamaBackend{}
	assert.Equal(t, "llama", lb.Name())
}

// --- Available ---

func TestLlamaBackend_Available_NoProcID_Bad(t *testing.T) {
	lb := &LlamaBackend{} // procID is ""
	assert.False(t, lb.Available(), "Available should return false when procID is empty")
}

func TestLlamaBackend_Available_HealthyServer_Good(t *testing.T) {
	srv := newMockLlamaServer(t, "unused")
	defer srv.Close()

	lb := &LlamaBackend{
		procID: "test-proc",
		port:   serverPort(srv),
	}

	assert.True(t, lb.Available())
}

func TestLlamaBackend_Available_UnreachableServer_Bad(t *testing.T) {
	lb := &LlamaBackend{
		procID: "test-proc",
		port:   19999, // nothing listening here
	}
	assert.False(t, lb.Available())
}

func TestLlamaBackend_Available_UnhealthyServer_Bad(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	lb := &LlamaBackend{
		procID: "test-proc",
		port:   serverPort(srv),
	}
	assert.False(t, lb.Available())
}

// --- Generate ---

func TestLlamaBackend_Generate_Good(t *testing.T) {
	srv := newMockLlamaServer(t, "generated response")
	defer srv.Close()

	lb := newLlamaBackendWithServer(srv)

	result, err := lb.Generate(context.Background(), "test prompt", DefaultGenOpts())
	require.NoError(t, err)
	assert.Equal(t, "generated response", result.Text)
	assert.Nil(t, result.Metrics)
}

func TestLlamaBackend_Generate_NotAvailable_Bad(t *testing.T) {
	lb := &LlamaBackend{
		procID: "",
		http:   NewHTTPBackend("http://127.0.0.1:19999", ""),
	}

	_, err := lb.Generate(context.Background(), "test", DefaultGenOpts())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestLlamaBackend_Generate_ServerError_Bad(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/chat/completions":
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad request"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	lb := newLlamaBackendWithServer(srv)

	_, err := lb.Generate(context.Background(), "test", DefaultGenOpts())
	require.Error(t, err)
}

// --- Chat ---

func TestLlamaBackend_Chat_Good(t *testing.T) {
	srv := newMockLlamaServer(t, "chat reply")
	defer srv.Close()

	lb := newLlamaBackendWithServer(srv)
	messages := []Message{
		{Role: "user", Content: "hello"},
	}

	result, err := lb.Chat(context.Background(), messages, DefaultGenOpts())
	require.NoError(t, err)
	assert.Equal(t, "chat reply", result.Text)
}

func TestLlamaBackend_Chat_MultiTurn_Good(t *testing.T) {
	srv := newMockLlamaServer(t, "multi-turn reply")
	defer srv.Close()

	lb := newLlamaBackendWithServer(srv)
	messages := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hi there"},
		{Role: "assistant", Content: "Hello!"},
		{Role: "user", Content: "How are you?"},
	}

	result, err := lb.Chat(context.Background(), messages, DefaultGenOpts())
	require.NoError(t, err)
	assert.Equal(t, "multi-turn reply", result.Text)
}

func TestLlamaBackend_Chat_NotAvailable_Bad(t *testing.T) {
	lb := &LlamaBackend{
		procID: "",
		http:   NewHTTPBackend("http://127.0.0.1:19999", ""),
	}

	messages := []Message{{Role: "user", Content: "hello"}}
	_, err := lb.Chat(context.Background(), messages, DefaultGenOpts())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

// --- Stop ---

func TestLlamaBackend_Stop_NoProcID_Good(t *testing.T) {
	lb := &LlamaBackend{} // procID is ""
	err := lb.Stop()
	assert.NoError(t, err, "Stop with empty procID should be a no-op")
}

// --- NewLlamaBackend constructor ---

func TestNewLlamaBackend_DefaultPort_Good(t *testing.T) {
	lb := NewLlamaBackend(nil, LlamaOpts{ModelPath: "/tmp/model.gguf"})

	assert.Equal(t, 18090, lb.port)
	assert.Equal(t, "/tmp/model.gguf", lb.modelPath)
	assert.Equal(t, "llama-server", lb.llamaPath)
	assert.NotNil(t, lb.http)
}

func TestNewLlamaBackend_CustomPort_Good(t *testing.T) {
	lb := NewLlamaBackend(nil, LlamaOpts{
		ModelPath: "/tmp/model.gguf",
		Port:      9999,
		LlamaPath: "/usr/local/bin/llama-server",
	})

	assert.Equal(t, 9999, lb.port)
	assert.Equal(t, "/usr/local/bin/llama-server", lb.llamaPath)
}

func TestNewLlamaBackend_WithLoRA_Good(t *testing.T) {
	lb := NewLlamaBackend(nil, LlamaOpts{
		ModelPath: "/tmp/model.gguf",
		LoraPath:  "/tmp/lora.gguf",
	})

	assert.Equal(t, "/tmp/lora.gguf", lb.loraPath)
}

func TestNewLlamaBackend_DefaultLlamaPath_Good(t *testing.T) {
	lb := NewLlamaBackend(nil, LlamaOpts{
		ModelPath: "/tmp/model.gguf",
		LlamaPath: "", // should default
	})
	assert.Equal(t, "llama-server", lb.llamaPath)
}

// --- Context cancellation ---

func TestLlamaBackend_Generate_ContextCancelled_Bad(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/chat/completions":
			// Block until client disconnects.
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	lb := newLlamaBackendWithServer(srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := lb.Generate(ctx, "test", DefaultGenOpts())
	require.Error(t, err)
}

// --- Empty choices edge case ---

func TestLlamaBackend_Generate_EmptyChoices_Ugly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/chat/completions":
			resp := chatResponse{Choices: []chatChoice{}}
			json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	lb := newLlamaBackendWithServer(srv)

	_, err := lb.Generate(context.Background(), "test", DefaultGenOpts())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no choices")
}

// --- GenOpts forwarding ---

func TestLlamaBackend_Generate_OptsForwarded_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/chat/completions":
			var req chatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode: %v", err)
			}
			// Verify opts were forwarded.
			assert.InDelta(t, 0.7, req.Temperature, 0.01)
			assert.Equal(t, 256, req.MaxTokens)

			resp := chatResponse{
				Choices: []chatChoice{{Message: Message{Role: "assistant", Content: "ok"}}},
			}
			json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	lb := newLlamaBackendWithServer(srv)

	opts := GenOpts{Temperature: 0.7, MaxTokens: 256}
	result, err := lb.Generate(context.Background(), "test", opts)
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Text)
}

// --- Verify Backend interface compliance ---

func TestLlamaBackend_InterfaceCompliance_Good(t *testing.T) {
	var _ Backend = (*LlamaBackend)(nil)
}
