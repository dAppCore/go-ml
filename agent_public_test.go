// SPDX-License-Identifier: EUPL-1.2

package ml

import (
	"context"
	"testing"
)

// ---------------------------------------------------------------------------
// Agent — spec §8 Agent orchestrator
// ---------------------------------------------------------------------------

func TestAgent_NewAgent_Good(t *testing.T) {
	cfg := &AgentConfig{
		M3Host:     "testhost",
		M3User:     "tester",
		APIURL:     "http://127.0.0.1:11434",
		JudgeURL:   "http://127.0.0.1:11434",
		JudgeModel: "qwen3:8b",
	}
	a := NewAgent(cfg)
	if a == nil {
		t.Fatal("NewAgent returned nil")
	}
	if a.Config() != cfg {
		t.Error("Config() did not return the supplied pointer")
	}
}

func TestAgent_ExecuteRemote_Good(t *testing.T) {
	ft := newFakeTransport()
	ft.On("echo hello", "hello\n", nil)

	cfg := &AgentConfig{Transport: ft}
	a := NewAgent(cfg)

	out, err := a.ExecuteRemote(context.Background(), "echo hello")
	if err != nil {
		t.Fatalf("ExecuteRemote err = %v", err)
	}
	if out != "hello\n" {
		t.Errorf("out = %q, want %q", out, "hello\n")
	}
}

func TestAgent_ExecuteRemote_Bad(t *testing.T) {
	ft := newFakeTransport() // no commands registered → "no match" error
	cfg := &AgentConfig{Transport: ft}
	a := NewAgent(cfg)

	if _, err := a.ExecuteRemote(context.Background(), "ls /"); err == nil {
		t.Error("expected error from fakeTransport with no registered patterns")
	}
}

func TestAgent_DiscoverCheckpoints_Ugly(t *testing.T) {
	// Empty transport output → no checkpoints, no error.
	ft := newFakeTransport()
	ft.On("ls -d", "", nil)

	cfg := &AgentConfig{Transport: ft, M3AdapterBase: "/tmp/adapters"}
	a := NewAgent(cfg)

	cps, err := a.DiscoverCheckpoints(context.Background())
	if err != nil {
		t.Fatalf("DiscoverCheckpoints err = %v", err)
	}
	if len(cps) != 0 {
		t.Errorf("expected 0 checkpoints, got %d", len(cps))
	}
}

func TestAgent_Influx_Good(t *testing.T) {
	cfg := &AgentConfig{InfluxURL: "http://localhost:8086", InfluxDB: "test"}
	a := NewAgent(cfg)
	if a.Influx() == nil {
		t.Fatal("Influx() returned nil")
	}
	// Second call returns the same cached client.
	if a.Influx() != a.Influx() {
		t.Error("Influx() did not cache client")
	}
}

// ---------------------------------------------------------------------------
// GGUF — spec §7 wrappers
// ---------------------------------------------------------------------------

func TestGGUF_ReadGGUFInfo_Bad(t *testing.T) {
	// Missing file must produce an error, not panic.
	if _, err := ReadGGUFInfo("/nonexistent/path/model.gguf"); err == nil {
		t.Error("expected error for missing GGUF file")
	}
}

func TestGGUF_DiscoverModels_Good(t *testing.T) {
	// Empty directory returns no models but does not panic.
	models := DiscoverModels(t.TempDir())
	if models == nil {
		// A nil slice is acceptable — callers treat it as len(models)==0.
		return
	}
	if len(models) != 0 {
		t.Errorf("expected 0 models in empty dir, got %d", len(models))
	}
}

func TestGGUF_DiscoverModels_Ugly(t *testing.T) {
	// Nonexistent path must not panic.
	_ = DiscoverModels("/definitely/not/a/real/path")
}
