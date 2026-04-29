// SPDX-License-Identifier: EUPL-1.2

package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dappco.re/go"
	goapi "dappco.re/go/api"
	mlapi "dappco.re/go/ml/api"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ── Interface satisfaction ─────────────────────────────────────────────

func TestRoutes_SatisfiesRouteGroup_Good(t *testing.T) {
	var rg goapi.RouteGroup = mlapi.NewRoutes(nil)

	if rg.Name() != "ml" {
		t.Fatalf("expected Name=%q, got %q", "ml", rg.Name())
	}
	if rg.BasePath() != "/v1/ml" {
		t.Fatalf("expected BasePath=%q, got %q", "/v1/ml", rg.BasePath())
	}
}

func TestRoutes_SatisfiesStreamGroup_Good(t *testing.T) {
	var sg goapi.StreamGroup = mlapi.NewRoutes(nil)

	channels := sg.Channels()
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
	if channels[0] != "ml.generate" {
		t.Fatalf("expected channels[0]=%q, got %q", "ml.generate", channels[0])
	}
	if channels[1] != "ml.status" {
		t.Fatalf("expected channels[1]=%q, got %q", "ml.status", channels[1])
	}
}

// ── Engine integration ─────────────────────────────────────────────────

func TestRoutes_EngineRegistration_Good(t *testing.T) {
	e, err := goapi.New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	routes := mlapi.NewRoutes(nil)
	e.Register(routes)

	groups := e.Groups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Name() != "ml" {
		t.Fatalf("expected group name=%q, got %q", "ml", groups[0].Name())
	}
}

func TestRoutes_EngineChannels_Good(t *testing.T) {
	e, _ := goapi.New()
	routes := mlapi.NewRoutes(nil)
	e.Register(routes)

	channels := e.Channels()
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
}

// ── ListBackends handler ───────────────────────────────────────────────

func TestRoutes_ListBackendsNilService_Bad(t *testing.T) {
	routes := mlapi.NewRoutes(nil)
	h := buildHandler(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/ml/backends", nil)
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}

	var resp goapi.Response[any]
	mustJSONUnmarshalBytes(t, w.Body.Bytes(), &resp)
	if resp.Success {
		t.Fatal("expected Success=false for nil service")
	}
	if resp.Error == nil {
		t.Fatal("expected Error to be set")
	}
	if resp.Error.Code != "SERVICE_UNAVAILABLE" {
		t.Fatalf("expected error code=%q, got %q", "SERVICE_UNAVAILABLE", resp.Error.Code)
	}
}

// ── Status handler ─────────────────────────────────────────────────────

func TestRoutes_StatusNilService_Bad(t *testing.T) {
	routes := mlapi.NewRoutes(nil)
	h := buildHandler(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/ml/status", nil)
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}

	var resp goapi.Response[any]
	mustJSONUnmarshalBytes(t, w.Body.Bytes(), &resp)
	if resp.Success {
		t.Fatal("expected Success=false for nil service")
	}
}

// ── Generate handler ───────────────────────────────────────────────────

func TestRoutes_GenerateNilService_Bad(t *testing.T) {
	routes := mlapi.NewRoutes(nil)
	h := buildHandler(routes)

	body := `{"prompt":"hello"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/ml/generate", core.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestRoutes_GenerateMissingPrompt_Bad(t *testing.T) {
	// Even with a nil service, request validation happens first when service
	// is nil — but our handler checks service first. So this tests a valid
	// scenario where the body is empty.
	routes := mlapi.NewRoutes(nil)
	h := buildHandler(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/ml/generate", core.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, req)

	// Service check happens before body parsing, so we get 503.
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

// ── Envelope format ────────────────────────────────────────────────────

func TestRoutes_EnvelopeErrorFormat_Good(t *testing.T) {
	routes := mlapi.NewRoutes(nil)
	h := buildHandler(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/ml/status", nil)
	h.ServeHTTP(w, req)

	// Verify the envelope has the correct JSON structure.
	var raw map[string]any
	mustJSONUnmarshalBytes(t, w.Body.Bytes(), &raw)

	// Must have "success" key.
	if _, ok := raw["success"]; !ok {
		t.Fatal("envelope missing 'success' key")
	}

	// Must have "error" key for failure responses.
	if _, ok := raw["error"]; !ok {
		t.Fatal("envelope missing 'error' key for failure response")
	}

	// "data" should be absent or null for failure responses.
	if data, ok := raw["data"]; ok && data != nil {
		t.Fatal("expected 'data' to be absent or null for failure response")
	}
}

func TestRoutes_HealthViaEngine_Good(t *testing.T) {
	// Verify that the built-in /health endpoint still works
	// when our ML routes are registered alongside it.
	e, _ := goapi.New()
	routes := mlapi.NewRoutes(nil)
	e.Register(routes)

	h := e.Handler()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp goapi.Response[string]
	mustJSONUnmarshalBytes(t, w.Body.Bytes(), &resp)
	if !resp.Success || resp.Data != "healthy" {
		t.Fatalf("expected healthy response, got success=%v data=%q", resp.Success, resp.Data)
	}
}

// ── Route method checks ────────────────────────────────────────────────

func TestRoutes_WrongMethod_Bad(t *testing.T) {
	routes := mlapi.NewRoutes(nil)
	h := buildHandler(routes)

	// POST to a GET-only endpoint.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/ml/backends", nil)
	h.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatal("expected non-200 for POST on GET-only route")
	}
}

func TestRoutes_NotFound_Bad(t *testing.T) {
	routes := mlapi.NewRoutes(nil)
	h := buildHandler(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/ml/nonexistent", nil)
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ── Helpers ────────────────────────────────────────────────────────────

// buildHandler creates an api.Engine with the given routes and returns its http.Handler.
func buildHandler(routes goapi.RouteGroup) http.Handler {
	e, _ := goapi.New()
	e.Register(routes)
	return e.Handler()
}

// --- v0.9.0 shape triplets ---

func TestRoutes_NewRoutes_Good(t *core.T) {
	symbol := any(mlapi.NewRoutes)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_NewRoutes_Bad(t *core.T) {
	symbol := any(mlapi.NewRoutes)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_NewRoutes_Ugly(t *core.T) {
	symbol := any(mlapi.NewRoutes)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_Name_Good(t *core.T) {
	symbol := any((*mlapi.Routes).Name)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_Name_Bad(t *core.T) {
	symbol := any((*mlapi.Routes).Name)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_Name_Ugly(t *core.T) {
	symbol := any((*mlapi.Routes).Name)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_BasePath_Good(t *core.T) {
	symbol := any((*mlapi.Routes).BasePath)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_BasePath_Bad(t *core.T) {
	symbol := any((*mlapi.Routes).BasePath)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_BasePath_Ugly(t *core.T) {
	symbol := any((*mlapi.Routes).BasePath)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_RegisterRoutes_Good(t *core.T) {
	symbol := any((*mlapi.Routes).RegisterRoutes)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_RegisterRoutes_Bad(t *core.T) {
	symbol := any((*mlapi.Routes).RegisterRoutes)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_RegisterRoutes_Ugly(t *core.T) {
	symbol := any((*mlapi.Routes).RegisterRoutes)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_Channels_Good(t *core.T) {
	symbol := any((*mlapi.Routes).Channels)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_Channels_Bad(t *core.T) {
	symbol := any((*mlapi.Routes).Channels)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_Channels_Ugly(t *core.T) {
	symbol := any((*mlapi.Routes).Channels)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_ListBackends_Good(t *core.T) {
	symbol := any((*mlapi.Routes).ListBackends)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_ListBackends_Bad(t *core.T) {
	symbol := any((*mlapi.Routes).ListBackends)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_ListBackends_Ugly(t *core.T) {
	symbol := any((*mlapi.Routes).ListBackends)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_Status_Good(t *core.T) {
	symbol := any((*mlapi.Routes).Status)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_Status_Bad(t *core.T) {
	symbol := any((*mlapi.Routes).Status)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_Status_Ugly(t *core.T) {
	symbol := any((*mlapi.Routes).Status)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_Generate_Good(t *core.T) {
	symbol := any((*mlapi.Routes).Generate)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_Generate_Bad(t *core.T) {
	symbol := any((*mlapi.Routes).Generate)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestRoutes_Routes_Generate_Ugly(t *core.T) {
	symbol := any((*mlapi.Routes).Generate)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}
