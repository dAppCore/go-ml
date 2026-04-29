// SPDX-License-Identifier: EUPL-1.2

package cmd

import (
	"context"
	"iter"
	"net/http"
	"net/http/httptest"
	"testing"

	"dappco.re/go"
	"dappco.re/go/inference"
	"dappco.re/go/ml"
)

type serveInfoTextModel struct {
	info    inference.ModelInfo
	metrics inference.GenerateMetrics
}

func (m *serveInfoTextModel) Generate(_ context.Context, _ string, _ ...inference.GenerateOption) iter.Seq[inference.Token] {
	return func(yield func(inference.Token) bool) {}
}

func (m *serveInfoTextModel) Chat(_ context.Context, _ []inference.Message, _ ...inference.GenerateOption) iter.Seq[inference.Token] {
	return func(yield func(inference.Token) bool) {}
}

func (m *serveInfoTextModel) Classify(_ context.Context, _ []string, _ ...inference.GenerateOption) ([]inference.ClassifyResult, error) {
	return nil, nil
}

func (m *serveInfoTextModel) BatchGenerate(_ context.Context, _ []string, _ ...inference.GenerateOption) ([]inference.BatchResult, error) {
	return nil, nil
}

func (m *serveInfoTextModel) ModelType() string                  { return m.info.Architecture }
func (m *serveInfoTextModel) Info() inference.ModelInfo          { return m.info }
func (m *serveInfoTextModel) Metrics() inference.GenerateMetrics { return m.metrics }
func (m *serveInfoTextModel) Err() error                         { return nil }
func (m *serveInfoTextModel) Close() error                       { return nil }

func TestServeModelInfo_Good(t *testing.T) {
	backend := ml.NewInferenceAdapter(&serveInfoTextModel{
		info: inference.ModelInfo{
			Architecture: "gemma3",
			VocabSize:    256000,
			NumLayers:    34,
			HiddenSize:   3072,
			QuantBits:    4,
			QuantGroup:   32,
		},
		metrics: inference.GenerateMetrics{
			PrefillTokensPerSec: 712.4,
			DecodeTokensPerSec:  82.6,
			PeakMemoryBytes:     8589934592,
			ActiveMemoryBytes:   6442450944,
		},
	}, "gemma3-4b-it-q4")

	mux := http.NewServeMux()
	registerModelRoutes(mux, backend)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models/gemma3-4b-it-q4/info", nil)
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var got modelInfoResponse
	if r := core.JSONUnmarshalString(w.Body.String(), &got); !r.OK {
		t.Fatalf("decode response: %v", r.Value)
	}

	if got.ID != "gemma3-4b-it-q4" {
		t.Fatalf("id = %q, want %q", got.ID, "gemma3-4b-it-q4")
	}
	if got.Architecture != "gemma3" || got.VocabSize != 256000 || got.NumLayers != 34 || got.HiddenSize != 3072 {
		t.Fatalf("model info = %+v", got)
	}
	if got.QuantBits != 4 || got.QuantGroup != 32 {
		t.Fatalf("quant info = %+v", got)
	}
	if got.PrefillTokensPerSec != 712.4 || got.DecodeTokensPerSec != 82.6 {
		t.Fatalf("throughput metrics = %+v", got)
	}
	if got.PeakMemoryBytes != 8589934592 || got.ActiveMemoryBytes != 6442450944 {
		t.Fatalf("memory metrics = %+v", got)
	}
}

func TestServeModelInfo_EncodedModelID_Good(t *testing.T) {
	backend := ml.NewInferenceAdapter(&serveInfoTextModel{
		info: inference.ModelInfo{Architecture: "gemma3"},
	}, "family/model q4")

	mux := http.NewServeMux()
	registerModelRoutes(mux, backend)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models/family%2Fmodel%20q4/info", nil)
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var got modelInfoResponse
	if r := core.JSONUnmarshalString(w.Body.String(), &got); !r.OK {
		t.Fatalf("decode response: %v", r.Value)
	}
	if got.ID != "family/model q4" {
		t.Fatalf("id = %q, want %q", got.ID, "family/model q4")
	}
}

func TestServeModelInfo_UnknownModel_Bad(t *testing.T) {
	backend := ml.NewInferenceAdapter(&serveInfoTextModel{}, "current")

	mux := http.NewServeMux()
	registerModelRoutes(mux, backend)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models/other/info", nil)
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
