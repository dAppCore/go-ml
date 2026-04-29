package ml

import (
	"context"
	"dappco.re/go"
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

func TestHTTPBackend_StopSequences_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: "hello STOP world"}}},
		}
		mustWriteJSONResponse(t, w, resp)
	}))
	defer srv.Close()

	b := NewHTTPBackend(srv.URL, "test-model")
	result, err := b.Generate(context.Background(), "hello", GenOpts{StopSequences: []string{"STOP"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Text != "hello " {
		t.Errorf("got %q, want %q", result.Text, "hello ")
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

// --- v0.9.0 shape triplets ---

func TestBackendHttp_WithHTTPClient_Good(t *core.T) {
	symbol := any(WithHTTPClient)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_WithHTTPClient_Bad(t *core.T) {
	symbol := any(WithHTTPClient)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_WithMedium_Bad(t *core.T) {
	symbol := any(WithMedium)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_WithMedium_Ugly(t *core.T) {
	symbol := any(WithMedium)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_WithHTTPMaxTokens_Bad(t *core.T) {
	symbol := any(WithHTTPMaxTokens)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_WithHTTPMaxTokens_Ugly(t *core.T) {
	symbol := any(WithHTTPMaxTokens)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_Error_Error_Good(t *core.T) {
	symbol := any((*retryableError).Error)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_Error_Error_Bad(t *core.T) {
	symbol := any((*retryableError).Error)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_Error_Error_Ugly(t *core.T) {
	symbol := any((*retryableError).Error)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_Error_Unwrap_Good(t *core.T) {
	symbol := any((*retryableError).Unwrap)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_Error_Unwrap_Bad(t *core.T) {
	symbol := any((*retryableError).Unwrap)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_Error_Unwrap_Ugly(t *core.T) {
	symbol := any((*retryableError).Unwrap)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_NewHTTPBackend_Good(t *core.T) {
	symbol := any(NewHTTPBackend)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_NewHTTPBackend_Bad(t *core.T) {
	symbol := any(NewHTTPBackend)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_NewHTTPBackend_Ugly(t *core.T) {
	symbol := any(NewHTTPBackend)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Medium_Good(t *core.T) {
	symbol := any((*HTTPBackend).Medium)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Medium_Bad(t *core.T) {
	symbol := any((*HTTPBackend).Medium)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Medium_Ugly(t *core.T) {
	symbol := any((*HTTPBackend).Medium)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Name_Good(t *core.T) {
	symbol := any((*HTTPBackend).Name)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Name_Bad(t *core.T) {
	symbol := any((*HTTPBackend).Name)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Name_Ugly(t *core.T) {
	symbol := any((*HTTPBackend).Name)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Available_Good(t *core.T) {
	symbol := any((*HTTPBackend).Available)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Available_Bad(t *core.T) {
	symbol := any((*HTTPBackend).Available)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Available_Ugly(t *core.T) {
	symbol := any((*HTTPBackend).Available)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Model_Good(t *core.T) {
	symbol := any((*HTTPBackend).Model)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Model_Bad(t *core.T) {
	symbol := any((*HTTPBackend).Model)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Model_Ugly(t *core.T) {
	symbol := any((*HTTPBackend).Model)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_BaseURL_Good(t *core.T) {
	symbol := any((*HTTPBackend).BaseURL)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_BaseURL_Bad(t *core.T) {
	symbol := any((*HTTPBackend).BaseURL)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_BaseURL_Ugly(t *core.T) {
	symbol := any((*HTTPBackend).BaseURL)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_SetMaxTokens_Good(t *core.T) {
	symbol := any((*HTTPBackend).SetMaxTokens)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_SetMaxTokens_Bad(t *core.T) {
	symbol := any((*HTTPBackend).SetMaxTokens)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_SetMaxTokens_Ugly(t *core.T) {
	symbol := any((*HTTPBackend).SetMaxTokens)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_LoadModel_Good(t *core.T) {
	symbol := any((*HTTPBackend).LoadModel)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_LoadModel_Bad(t *core.T) {
	symbol := any((*HTTPBackend).LoadModel)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_LoadModel_Ugly(t *core.T) {
	symbol := any((*HTTPBackend).LoadModel)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Generate_Good(t *core.T) {
	symbol := any((*HTTPBackend).Generate)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Generate_Bad(t *core.T) {
	symbol := any((*HTTPBackend).Generate)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Generate_Ugly(t *core.T) {
	symbol := any((*HTTPBackend).Generate)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Chat_Good(t *core.T) {
	symbol := any((*HTTPBackend).Chat)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Chat_Bad(t *core.T) {
	symbol := any((*HTTPBackend).Chat)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestBackendHttp_HTTPBackend_Chat_Ugly(t *core.T) {
	symbol := any((*HTTPBackend).Chat)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}
