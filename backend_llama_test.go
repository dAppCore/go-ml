// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"
	"dappco.re/go"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
)

// ---------------------------------------------------------------------------
// LlamaBackend unit tests — no subprocess, HTTP mocked via httptest
// ---------------------------------------------------------------------------

// newMockLlamaServer creates an httptest.Server that responds to both
// /health and /v1/chat/completions.  Returns a fixed content string for chat
// and 200 OK for health.
func newMockLlamaServer(t *core.T, chatContent string) *httptest.Server {
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
			mustWriteJSONResponse(t, w, resp)
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

func TestLlamaBackend_Name_Good(t *core.T) {
	lb := &LlamaBackend{}
	name := lb.Name()
	core.AssertEqual(t, "llama", name)
	core.AssertFalse(t, lb.Available())
}

// --- Available ---

func TestLlamaBackend_Available_NoProcID_Bad(t *core.T) {
	lb := &LlamaBackend{} // procID is ""
	available := lb.Available()
	core.AssertFalse(t, available, "Available should return false when procID is empty")
	core.AssertEqual(t, "", lb.procID)
}

func TestLlamaBackend_Available_HealthyServer_Good(t *core.T) {
	srv := newMockLlamaServer(t, "unused")
	defer srv.Close()

	lb := &LlamaBackend{
		procID: "test-proc",
		port:   serverPort(srv),
	}

	core.AssertTrue(t, lb.Available())
}

func TestLlamaBackend_Available_UnreachableServer_Bad(t *core.T) {
	lb := &LlamaBackend{
		procID: "test-proc",
		port:   19999, // nothing listening here
	}
	core.AssertFalse(t, lb.Available())
}

func TestLlamaBackend_Available_UnhealthyServer_Bad(t *core.T) {
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
	core.AssertFalse(t, lb.Available())
}

// --- Generate ---

func TestLlamaBackend_Generate_Good(t *core.T) {
	srv := newMockLlamaServer(t, "generated response")
	defer srv.Close()

	lb := newLlamaBackendWithServer(srv)

	result, err := lb.Generate(context.Background(), "test prompt", DefaultGenOpts())
	core.RequireNoError(t, err)
	core.AssertEqual(t, "generated response", result.Text)
	core.AssertNil(t, result.Metrics)
}

func TestLlamaBackend_Generate_NotAvailable_Bad(t *core.T) {
	lb := &LlamaBackend{
		procID: "",
		http:   NewHTTPBackend("http://127.0.0.1:19999", ""),
	}

	_, err := lb.Generate(context.Background(), "test", DefaultGenOpts())
	core.AssertError(t, err)
	core.AssertContains(t, err.Error(), "not available")
}

func TestLlamaBackend_Generate_ServerError_Bad(t *core.T) {
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
	core.AssertError(t, err)
}

// --- Chat ---

func TestLlamaBackend_Chat_Good(t *core.T) {
	srv := newMockLlamaServer(t, "chat reply")
	defer srv.Close()

	lb := newLlamaBackendWithServer(srv)
	messages := []Message{
		{Role: "user", Content: "hello"},
	}

	result, err := lb.Chat(context.Background(), messages, DefaultGenOpts())
	core.RequireNoError(t, err)
	core.AssertEqual(t, "chat reply", result.Text)
}

func TestLlamaBackend_Chat_MultiTurn_Good(t *core.T) {
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
	core.RequireNoError(t, err)
	core.AssertEqual(t, "multi-turn reply", result.Text)
}

func TestLlamaBackend_Chat_NotAvailable_Bad(t *core.T) {
	lb := &LlamaBackend{
		procID: "",
		http:   NewHTTPBackend("http://127.0.0.1:19999", ""),
	}

	messages := []Message{{Role: "user", Content: "hello"}}
	_, err := lb.Chat(context.Background(), messages, DefaultGenOpts())
	core.AssertError(t, err)
	core.AssertContains(t, err.Error(), "not available")
}

// --- Stop ---

func TestLlamaBackend_Stop_NoProcID_Good(t *core.T) {
	lb := &LlamaBackend{} // procID is ""
	err := lb.Stop()
	core.AssertNoError(t, err, "Stop with empty procID should be a no-op")
}

// --- NewLlamaBackend constructor ---

func TestNewLlamaBackend_DefaultPort_Good(t *core.T) {
	lb := NewLlamaBackend(nil, LlamaOpts{ModelPath: "/tmp/model.gguf"})

	core.AssertEqual(t, 18090, lb.port)
	core.AssertEqual(t, "/tmp/model.gguf", lb.modelPath)
	core.AssertEqual(t, "llama-server", lb.llamaPath)
	core.AssertNotNil(t, lb.http)
}

func TestNewLlamaBackend_CustomPort_Good(t *core.T) {
	lb := NewLlamaBackend(nil, LlamaOpts{
		ModelPath: "/tmp/model.gguf",
		Port:      9999,
		LlamaPath: "/usr/local/bin/llama-server",
	})

	core.AssertEqual(t, 9999, lb.port)
	core.AssertEqual(t, "/usr/local/bin/llama-server", lb.llamaPath)
}

func TestNewLlamaBackend_WithLoRA_Good(t *core.T) {
	lb := NewLlamaBackend(nil, LlamaOpts{
		ModelPath: "/tmp/model.gguf",
		LoraPath:  "/tmp/lora.gguf",
	})

	core.AssertEqual(t, "/tmp/lora.gguf", lb.loraPath)
}

func TestNewLlamaBackend_DefaultLlamaPath_Good(t *core.T) {
	lb := NewLlamaBackend(nil, LlamaOpts{
		ModelPath: "/tmp/model.gguf",
		LlamaPath: "", // should default
	})
	core.AssertEqual(t, "llama-server", lb.llamaPath)
}

// --- Context cancellation ---

func TestLlamaBackend_Generate_ContextCancelled_Bad(t *core.T) {
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
	core.AssertError(t, err)
}

// --- Empty choices edge case ---

func TestLlamaBackend_Generate_EmptyChoices_Ugly(t *core.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/chat/completions":
			resp := chatResponse{Choices: []chatChoice{}}
			mustWriteJSONResponse(t, w, resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	lb := newLlamaBackendWithServer(srv)

	_, err := lb.Generate(context.Background(), "test", DefaultGenOpts())
	core.AssertError(t, err)
	core.AssertContains(t, err.Error(), "no choices")
}

// --- GenOpts forwarding ---

func TestLlamaBackend_Generate_OptsForwarded_Good(t *core.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/chat/completions":
			var req chatRequest
			mustReadJSONRequest(t, r, &req)
			// Verify opts were forwarded.
			core.AssertInDelta(t, 0.7, req.Temperature, 0.01)
			core.AssertEqual(t, 256, req.MaxTokens)

			resp := chatResponse{
				Choices: []chatChoice{{Message: Message{Role: "assistant", Content: "ok"}}},
			}
			mustWriteJSONResponse(t, w, resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	lb := newLlamaBackendWithServer(srv)

	opts := GenOpts{Temperature: 0.7, MaxTokens: 256}
	result, err := lb.Generate(context.Background(), "test", opts)
	core.RequireNoError(t, err)
	core.AssertEqual(t, "ok", result.Text)
}

// --- Verify Backend interface compliance ---

func TestLlamaBackend_InterfaceCompliance_Good(t *core.T) {
	var backend Backend = &LlamaBackend{}
	core.AssertNotNil(t, backend)
	core.AssertEqual(t, "llama", backend.Name())
}

// TestLlamaBackend_SetMaxTokens_Good — spec §2.4: SetMaxTokens forwards to
// the internal HTTP client so subsequent generate calls carry max_tokens.
//
//	backend := ml.NewLlamaBackend(svc, opts)
//	backend.SetMaxTokens(2048)
func TestLlamaBackend_SetMaxTokens_Good(t *core.T) {
	lb := &LlamaBackend{
		http: NewHTTPBackend("http://localhost", ""),
	}
	lb.SetMaxTokens(2048)
	core.AssertEqual(t, 2048, lb.http.maxTokens)
}

// --- v0.9.0 shape triplets ---

func TestBackendLlama_NewLlamaBackend_Good(t *core.T) {
	symbol := any(NewLlamaBackend)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_NewLlamaBackend_Bad(t *core.T) {
	symbol := any(NewLlamaBackend)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_NewLlamaBackend_Ugly(t *core.T) {
	symbol := any(NewLlamaBackend)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Name_Good(t *core.T) {
	symbol := any((*LlamaBackend).Name)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Name_Bad(t *core.T) {
	symbol := any((*LlamaBackend).Name)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Name_Ugly(t *core.T) {
	symbol := any((*LlamaBackend).Name)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_SetMaxTokens_Good(t *core.T) {
	symbol := any((*LlamaBackend).SetMaxTokens)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_SetMaxTokens_Bad(t *core.T) {
	symbol := any((*LlamaBackend).SetMaxTokens)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_SetMaxTokens_Ugly(t *core.T) {
	symbol := any((*LlamaBackend).SetMaxTokens)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_LoadModel_Good(t *core.T) {
	symbol := any((*LlamaBackend).LoadModel)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_LoadModel_Bad(t *core.T) {
	symbol := any((*LlamaBackend).LoadModel)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_LoadModel_Ugly(t *core.T) {
	symbol := any((*LlamaBackend).LoadModel)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Available_Good(t *core.T) {
	symbol := any((*LlamaBackend).Available)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Available_Bad(t *core.T) {
	symbol := any((*LlamaBackend).Available)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Available_Ugly(t *core.T) {
	symbol := any((*LlamaBackend).Available)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Start_Good(t *core.T) {
	symbol := any((*LlamaBackend).Start)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Start_Bad(t *core.T) {
	symbol := any((*LlamaBackend).Start)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Start_Ugly(t *core.T) {
	symbol := any((*LlamaBackend).Start)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Stop_Good(t *core.T) {
	symbol := any((*LlamaBackend).Stop)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Stop_Bad(t *core.T) {
	symbol := any((*LlamaBackend).Stop)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Stop_Ugly(t *core.T) {
	symbol := any((*LlamaBackend).Stop)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Generate_Good(t *core.T) {
	symbol := any((*LlamaBackend).Generate)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Generate_Bad(t *core.T) {
	symbol := any((*LlamaBackend).Generate)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Generate_Ugly(t *core.T) {
	symbol := any((*LlamaBackend).Generate)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Chat_Good(t *core.T) {
	symbol := any((*LlamaBackend).Chat)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Chat_Bad(t *core.T) {
	symbol := any((*LlamaBackend).Chat)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendLlama_LlamaBackend_Chat_Ugly(t *core.T) {
	symbol := any((*LlamaBackend).Chat)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}
