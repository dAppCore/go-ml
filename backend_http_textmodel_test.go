// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"dappco.re/go/core/inference"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer creates an httptest.Server that responds with the given content.
func newTestServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: content}}},
		}
		mustWriteJSONResponse(t, w, resp)
	}))
}

// newTestServerMulti creates an httptest.Server that responds based on the prompt.
func newTestServerMulti(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		mustReadJSONRequest(t, r, &req)
		// Echo back the last message content with a prefix.
		lastContent := ""
		if len(req.Messages) > 0 {
			lastContent = req.Messages[len(req.Messages)-1].Content
		}
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: "reply:" + lastContent}}},
		}
		mustWriteJSONResponse(t, w, resp)
	}))
}

func TestHTTPTextModel_Generate_Good(t *testing.T) {
	srv := newTestServer(t, "Hello from HTTP")
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "test-model")
	model := NewHTTPTextModel(backend)

	var collected []inference.Token
	for tok := range model.Generate(context.Background(), "test prompt") {
		collected = append(collected, tok)
	}

	require.Len(t, collected, 1)
	assert.Equal(t, "Hello from HTTP", collected[0].Text)
	assert.NoError(t, model.Err())
}

func TestHTTPTextModel_Generate_WithOpts_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		mustReadJSONRequest(t, r, &req)

		// Verify that options are passed through.
		assert.InDelta(t, 0.8, req.Temperature, 0.01)
		assert.Equal(t, 100, req.MaxTokens)

		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: "configured"}}},
		}
		mustWriteJSONResponse(t, w, resp)
	}))
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "test-model")
	model := NewHTTPTextModel(backend)

	var result string
	for tok := range model.Generate(context.Background(), "prompt",
		inference.WithTemperature(0.8),
		inference.WithMaxTokens(100),
	) {
		result = tok.Text
	}
	assert.Equal(t, "configured", result)
	assert.NoError(t, model.Err())
}

func TestHTTPTextModel_Chat_Good(t *testing.T) {
	srv := newTestServer(t, "chat response")
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "test-model")
	model := NewHTTPTextModel(backend)

	messages := []inference.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
	}

	var collected []inference.Token
	for tok := range model.Chat(context.Background(), messages) {
		collected = append(collected, tok)
	}

	require.Len(t, collected, 1)
	assert.Equal(t, "chat response", collected[0].Text)
	assert.NoError(t, model.Err())
}

func TestHTTPTextModel_Generate_Error_Bad(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request"))
	}))
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "test-model")
	model := NewHTTPTextModel(backend)

	var collected []inference.Token
	for tok := range model.Generate(context.Background(), "bad prompt") {
		collected = append(collected, tok)
	}

	assert.Empty(t, collected, "no tokens should be yielded on error")
	assert.Error(t, model.Err())
	assert.Contains(t, model.Err().Error(), "400")
}

func TestHTTPTextModel_Chat_Error_Bad(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad chat"))
	}))
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "test-model")
	model := NewHTTPTextModel(backend)

	messages := []inference.Message{{Role: "user", Content: "test"}}
	var collected []inference.Token
	for tok := range model.Chat(context.Background(), messages) {
		collected = append(collected, tok)
	}

	assert.Empty(t, collected)
	assert.Error(t, model.Err())
}

func TestHTTPTextModel_Classify_Bad(t *testing.T) {
	backend := NewHTTPBackend("http://localhost", "test-model")
	model := NewHTTPTextModel(backend)

	results, err := model.Classify(context.Background(), []string{"test"})
	assert.Nil(t, results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "classify not supported")
}

func TestHTTPTextModel_BatchGenerate_Good(t *testing.T) {
	srv := newTestServerMulti(t)
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "test-model")
	model := NewHTTPTextModel(backend)

	prompts := []string{"alpha", "beta", "gamma"}
	results, err := model.BatchGenerate(context.Background(), prompts)
	require.NoError(t, err)
	require.Len(t, results, 3)

	assert.Equal(t, "reply:alpha", results[0].Tokens[0].Text)
	assert.NoError(t, results[0].Err)

	assert.Equal(t, "reply:beta", results[1].Tokens[0].Text)
	assert.NoError(t, results[1].Err)

	assert.Equal(t, "reply:gamma", results[2].Tokens[0].Text)
	assert.NoError(t, results[2].Err)
}

func TestHTTPTextModel_BatchGenerate_PartialError_Bad(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 2 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("error on second"))
			return
		}
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: "ok"}}},
		}
		mustWriteJSONResponse(t, w, resp)
	}))
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "test-model")
	model := NewHTTPTextModel(backend)

	results, err := model.BatchGenerate(context.Background(), []string{"a", "b", "c"})
	require.NoError(t, err) // BatchGenerate itself doesn't fail.
	require.Len(t, results, 3)

	assert.Len(t, results[0].Tokens, 1)
	assert.NoError(t, results[0].Err)

	assert.Empty(t, results[1].Tokens)
	assert.Error(t, results[1].Err, "second prompt should have an error")

	assert.Len(t, results[2].Tokens, 1)
	assert.NoError(t, results[2].Err)
}

func TestHTTPTextModel_ModelType_Good(t *testing.T) {
	backend := NewHTTPBackend("http://localhost", "gpt-4o")
	model := NewHTTPTextModel(backend)
	assert.Equal(t, "gpt-4o", model.ModelType())
}

func TestHTTPTextModel_ModelType_Empty_Good(t *testing.T) {
	backend := NewHTTPBackend("http://localhost", "")
	model := NewHTTPTextModel(backend)
	assert.Equal(t, "http", model.ModelType())
}

func TestHTTPTextModel_Info_Good(t *testing.T) {
	backend := NewHTTPBackend("http://localhost", "test")
	model := NewHTTPTextModel(backend)
	info := model.Info()
	assert.Equal(t, "http", info.Architecture)
}

func TestHTTPTextModel_Metrics_Good(t *testing.T) {
	backend := NewHTTPBackend("http://localhost", "test")
	model := NewHTTPTextModel(backend)
	metrics := model.Metrics()
	assert.Equal(t, inference.GenerateMetrics{}, metrics)
}

func TestHTTPTextModel_Err_ClearedOnSuccess_Good(t *testing.T) {
	// First request fails.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("fail"))
			return
		}
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: "ok"}}},
		}
		mustWriteJSONResponse(t, w, resp)
	}))
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "test-model")
	model := NewHTTPTextModel(backend)

	// First call: error.
	for range model.Generate(context.Background(), "fail") {
	}
	assert.Error(t, model.Err())

	// Second call: success — error should be cleared.
	for range model.Generate(context.Background(), "ok") {
	}
	assert.NoError(t, model.Err())
}

func TestHTTPTextModel_Close_Good(t *testing.T) {
	backend := NewHTTPBackend("http://localhost", "test")
	model := NewHTTPTextModel(backend)
	assert.NoError(t, model.Close())
}

func TestLlamaTextModel_ModelType_Good(t *testing.T) {
	// LlamaBackend requires a process.Service but we only test ModelType here.
	llama := &LlamaBackend{
		http: NewHTTPBackend("http://127.0.0.1:18090", ""),
	}
	model := NewLlamaTextModel(llama)
	assert.Equal(t, "llama", model.ModelType())
}

func TestLlamaTextModel_Close_Good(t *testing.T) {
	// LlamaBackend with no procID — Stop() is a no-op.
	llama := &LlamaBackend{
		http: NewHTTPBackend("http://127.0.0.1:18090", ""),
	}
	model := NewLlamaTextModel(llama)
	assert.NoError(t, model.Close())
}
