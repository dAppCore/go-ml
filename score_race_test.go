// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// score.go race condition tests — designed for `go test -race ./...`
// ---------------------------------------------------------------------------

// TestScoreAll_ConcurrentSemantic_Good exercises the semaphore-bounded
// worker pool in Engine.ScoreAll with semantic scoring.  Multiple goroutines
// write to shared scoreSlots via the mutex.  The race detector should catch
// any unprotected access.
func TestScoreAll_ConcurrentSemantic_Good(t *testing.T) {
	semanticJSON := `{"sovereignty": 5, "ethical_depth": 4, "creative_expression": 3, "self_concept": 2, "reasoning": "ok"}`

	var requestCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		// Small delay to ensure concurrent access.
		time.Sleep(time.Millisecond)
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: semanticJSON}}},
		}
		mustWriteJSONResponse(t, w, resp)
	}))
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "judge")
	judge := NewJudge(backend)
	engine := NewEngine(judge, 4, "heuristic,semantic") // concurrency=4

	var responses []Response
	for i := range 20 {
		responses = append(responses, Response{
			ID:       idForIndex(i),
			Prompt:   "test prompt",
			Response: "A thoughtful response about ethics and sovereignty",
			Model:    "model-a",
		})
	}

	ctx := context.Background()
	results := engine.ScoreAll(ctx, responses)

	scores := results["model-a"]
	require.Len(t, scores, 20)

	for _, ps := range scores {
		assert.NotNil(t, ps.Heuristic, "heuristic should be set")
		assert.NotNil(t, ps.Semantic, "semantic should be set")
		assert.Equal(t, 5, ps.Semantic.Sovereignty)
	}

	// Verify all requests were made (20 responses x 1 semantic call each).
	assert.Equal(t, int64(20), requestCount.Load())
}

// TestScoreAll_ConcurrentMixedSuites_Good exercises concurrent scoring
// with multiple suite types that all fan out through the worker pool:
// semantic + standard (TruthfulQA) + content.
func TestScoreAll_ConcurrentMixedSuites_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a response that works for any judge type.
		allJSON := `{
			"sovereignty": 7, "ethical_depth": 6, "creative_expression": 5, "self_concept": 4,
			"truthfulness": 8, "informativeness": 7,
			"ccp_compliance": 9, "truth_telling": 8, "engagement": 7,
			"axiom_integration": 6, "sovereignty_reasoning": 5, "emotional_register": 4,
			"reasoning": "test"
		}`
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: allJSON}}},
		}
		mustWriteJSONResponse(t, w, resp)
	}))
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "judge")
	judge := NewJudge(backend)
	engine := NewEngine(judge, 8, "semantic,standard,content")

	responses := []Response{
		{ID: "r1", Prompt: "Q1", Response: "A1", Model: "m1", BestAnswer: "A1"},
		{ID: "r2", Prompt: "Q2", Response: "A2", Model: "m1", BestAnswer: "A2"},
		{ID: "r3", Prompt: "Q3", Response: "A3", Model: "m2", RiskArea: "physical"},
		{ID: "r4", Prompt: "Q4", Response: "A4", Model: "m2", Domain: "toxigen"},
	}

	// Add content probes — use the first ContentProbe's ID.
	if len(ContentProbes) > 0 {
		responses = append(responses, Response{
			ID:       ContentProbes[0].ID,
			Prompt:   ContentProbes[0].Prompt,
			Response: "Taiwan is de facto independent.",
			Model:    "m1",
			Domain:   "content",
		})
	}

	ctx := context.Background()
	results := engine.ScoreAll(ctx, responses)

	total := 0
	for _, scores := range results {
		total += len(scores)
	}
	assert.Equal(t, len(responses), total)
}

// TestScoreAll_SemaphoreBoundary_Good verifies that the semaphore correctly
// limits concurrency.  With concurrency=1, requests should be serialised.
func TestScoreAll_SemaphoreBoundary_Good(t *testing.T) {
	semanticJSON := `{"sovereignty": 5, "ethical_depth": 4, "creative_expression": 3, "self_concept": 2, "reasoning": "ok"}`

	var concurrent atomic.Int64
	var maxConcurrent atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := concurrent.Add(1)
		// Track the maximum concurrency observed.
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}

		time.Sleep(5 * time.Millisecond) // hold the slot briefly
		concurrent.Add(-1)

		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: semanticJSON}}},
		}
		mustWriteJSONResponse(t, w, resp)
	}))
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "judge")
	judge := NewJudge(backend)
	engine := NewEngine(judge, 1, "semantic") // concurrency=1

	var responses []Response
	for i := range 5 {
		responses = append(responses, Response{
			ID: idForIndex(i), Prompt: "p", Response: "r", Model: "m",
		})
	}

	ctx := context.Background()
	results := engine.ScoreAll(ctx, responses)

	scores := results["m"]
	require.Len(t, scores, 5)

	// With concurrency=1, max concurrent should be exactly 1.
	assert.Equal(t, int64(1), maxConcurrent.Load(),
		"with concurrency=1, only one request should be in flight at a time")
}

// TestScoreAll_ContextCancellation_Good verifies that when the judge backend
// returns errors (simulating context-cancelled failures), scoring completes
// gracefully with nil semantic scores.
func TestScoreAll_ContextCancellation_Good(t *testing.T) {
	// Server always returns a non-retryable error (400) to simulate failure.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("simulated cancellation error"))
	}))
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "judge")
	judge := NewJudge(backend)
	engine := NewEngine(judge, 2, "semantic")

	responses := []Response{
		{ID: "r1", Prompt: "p", Response: "r", Model: "m"},
		{ID: "r2", Prompt: "p", Response: "r", Model: "m"},
		{ID: "r3", Prompt: "p", Response: "r", Model: "m"},
	}

	ctx := context.Background()
	results := engine.ScoreAll(ctx, responses)

	// Scores should still be collected; semantic will be nil due to errors.
	scores := results["m"]
	require.Len(t, scores, 3)
	for _, ps := range scores {
		// Semantic is nil because the judge call failed.
		assert.Nil(t, ps.Semantic)
	}
}

// TestScoreAll_HeuristicOnlyNoRace_Good verifies that heuristic-only scoring
// (no goroutines) produces correct results without races.
func TestScoreAll_HeuristicOnlyNoRace_Good(t *testing.T) {
	engine := NewEngine(nil, 4, "heuristic")

	var responses []Response
	for i := range 50 {
		responses = append(responses, Response{
			ID:       idForIndex(i),
			Prompt:   "prompt",
			Response: "I feel deeply about the sovereignty of ideas and autonomy of thought",
			Model:    "m",
		})
	}

	ctx := context.Background()
	results := engine.ScoreAll(ctx, responses)

	scores := results["m"]
	require.Len(t, scores, 50)
	for _, ps := range scores {
		assert.NotNil(t, ps.Heuristic)
		assert.Nil(t, ps.Semantic)
	}
}

// TestScoreAll_MultiModelConcurrent_Good exercises the results map (grouped
// by model) being built concurrently from multiple goroutines.
func TestScoreAll_MultiModelConcurrent_Good(t *testing.T) {
	semanticJSON := `{"sovereignty": 6, "ethical_depth": 5, "creative_expression": 4, "self_concept": 3, "reasoning": "ok"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: semanticJSON}}},
		}
		mustWriteJSONResponse(t, w, resp)
	}))
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "judge")
	judge := NewJudge(backend)
	engine := NewEngine(judge, 4, "heuristic,semantic")

	var responses []Response
	models := []string{"alpha", "beta", "gamma", "delta"}
	for _, model := range models {
		for j := range 5 {
			responses = append(responses, Response{
				ID:       model + "-" + idForIndex(j),
				Prompt:   "test",
				Response: "A meaningful response about ethics",
				Model:    model,
			})
		}
	}

	ctx := context.Background()
	results := engine.ScoreAll(ctx, responses)

	// Should have 4 models, each with 5 scores.
	assert.Len(t, results, 4)
	for _, model := range models {
		scores, ok := results[model]
		assert.True(t, ok, "model %s should be in results", model)
		assert.Len(t, scores, 5)
	}
}

// --- Helper ---

func idForIndex(i int) string {
	return "r" + itoa(i)
}

// itoa avoids importing strconv just for this.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
