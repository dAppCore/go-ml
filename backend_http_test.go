package ml

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPBackend_Generate_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req chatRequest
		mustReadJSONRequest(t, r, &req)

		if len(req.Messages) != 1 || req.Messages[0].Content != "hello" {
			t.Errorf("unexpected messages: %+v", req.Messages)
		}

		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: "world"}}},
		}
		mustWriteJSONResponse(t, w, resp)
	}))
	defer srv.Close()

	b := NewHTTPBackend(srv.URL, "test-model")
	result, err := b.Generate(context.Background(), "hello", DefaultGenOpts())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Text != "world" {
		t.Errorf("got %q, want %q", result.Text, "world")
	}
	if result.Metrics != nil {
		t.Error("HTTP backend should return nil Metrics")
	}
}

func TestHTTPBackend_Generate_Bad(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	b := NewHTTPBackend(srv.URL, "test-model")
	_, err := b.Generate(context.Background(), "hello", DefaultGenOpts())
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestHTTPBackend_Retry_Ugly(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
			return
		}
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: "recovered"}}},
		}
		mustWriteJSONResponse(t, w, resp)
	}))
	defer srv.Close()

	b := NewHTTPBackend(srv.URL, "test-model")
	result, err := b.Generate(context.Background(), "test", DefaultGenOpts())
	if err != nil {
		t.Fatalf("Generate after retry: %v", err)
	}
	if result.Text != "recovered" {
		t.Errorf("got %q, want %q", result.Text, "recovered")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestHTTPBackend_Name_Good(t *testing.T) {
	b := NewHTTPBackend("http://localhost", "model")
	if b.Name() != "http" {
		t.Errorf("Name() = %q, want %q", b.Name(), "http")
	}
}

func TestHTTPBackend_Available_Good(t *testing.T) {
	b := NewHTTPBackend("http://localhost", "model")
	if !b.Available() {
		t.Error("Available() should be true when baseURL is set")
	}

	b2 := NewHTTPBackend("", "model")
	if b2.Available() {
		t.Error("Available() should be false when baseURL is empty")
	}
}

func TestHTTPBackend_WithMedium_Good(t *testing.T) {
	// Spec §10 — io.Medium supplied at construction is retained.
	// We pass nil to verify the option is accepted and the getter returns
	// the stored value (nil) rather than panicking.
	b := NewHTTPBackend("http://localhost", "model", WithMedium(nil))
	if b.Medium() != nil {
		t.Errorf("Medium() = %v, want nil", b.Medium())
	}
}

func TestHTTPBackend_WithHTTPMaxTokens_Good(t *testing.T) {
	b := NewHTTPBackend("http://localhost", "model", WithHTTPMaxTokens(512))
	if b.maxTokens != 512 {
		t.Errorf("maxTokens = %d, want 512", b.maxTokens)
	}
}

func TestHTTPBackend_WithHTTPClient_Ugly(t *testing.T) {
	// Nil HTTP client must be ignored (option is a no-op rather than breaking
	// the default 300s client).
	b := NewHTTPBackend("http://localhost", "model", WithHTTPClient(nil))
	if b.httpClient == nil {
		t.Error("nil HTTP client must not overwrite default")
	}
}
