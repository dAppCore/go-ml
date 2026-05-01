package ml

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"dappco.re/go"
	coreio "dappco.re/go/io"
	"dappco.re/go/store"
)

type fakeInfluxRecorder struct {
	mu     sync.Mutex
	writes []string
}

func newFakeInflux(t testing.TB, queries map[string][]map[string]any, writeStatus int) (*InfluxClient, *fakeInfluxRecorder) {
	t.Helper()
	rec := &fakeInfluxRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/write_lp":
			body, _ := readAll(r.Body)
			rec.mu.Lock()
			rec.writes = append(rec.writes, string(body))
			rec.mu.Unlock()
			if writeStatus == 0 {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(writeStatus)
		case "/api/v3/query_sql":
			body, _ := readAll(r.Body)
			sql := string(body)
			rows := []map[string]any{}
			for key, value := range queries {
				if core.Contains(sql, key) {
					rows = value
					break
				}
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(core.JSONMarshalString(rows)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return &InfluxClient{url: server.URL, db: "test"}, rec
}

func (r *fakeInfluxRecorder) writeCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.writes)
}

func newTestDB(t testing.TB) *DB {
	t.Helper()
	db, err := OpenDBReadWrite(core.JoinPath(t.TempDir(), "test.duckdb"))
	core.RequireNoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newStoreDuckDB(t testing.TB) *store.DuckDB {
	t.Helper()
	db, err := store.OpenDuckDBReadWrite(core.JoinPath(t.TempDir(), "store.duckdb"))
	core.RequireNoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func writeSafetensorsFixture(t testing.TB) (string, string) {
	t.Helper()
	dir := t.TempDir()
	key := "model.layers.0.self_attn.q_proj.lora_a"
	sf := core.JoinPath(dir, "adapter_model.safetensors")
	cfg := core.JoinPath(dir, "adapter_config.json")
	tensors := map[string]SafetensorsTensorInfo{
		key: {Dtype: "F32", Shape: []int{1, 1}},
	}
	data := map[string][]byte{
		key: {1, 2, 3, 4},
	}
	core.RequireNoError(t, WriteSafetensors(sf, tensors, data))
	core.RequireNoError(t, coreio.Local.Write(cfg, `{"lora_parameters":{"rank":2,"scale":3,"dropout":0.1}}`))
	return sf, cfg
}

func sampleCheckpoint() Checkpoint {
	return Checkpoint{
		RemoteDir: "/remote/adapters",
		Filename:  "0000010_adapters.safetensors",
		Dirname:   "adapters-1b",
		Iteration: 10,
		ModelTag:  "gemma-3-1b",
		Label:     "G1 @10",
		RunID:     "g1-capability-auto",
	}
}

func sampleProbeResult() ProbeResult {
	return ProbeResult{
		Accuracy: 100,
		Correct:  1,
		Total:    1,
		ByCategory: map[string]CategoryResult{
			"arithmetic": {Correct: 1, Total: 1},
		},
		Probes: map[string]SingleProbeResult{
			"p1": {Passed: true, Response: "ok"},
		},
	}
}
