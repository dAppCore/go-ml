// SPDX-License-Identifier: EUPL-1.2

package ml

import (
	"context"

	"dappco.re/go"
)

// ---------------------------------------------------------------------------
// Agent — spec §8 Agent orchestrator
// ---------------------------------------------------------------------------

func TestAgentNewAgentStoresConfigScenario(t *core.T) {
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

func TestAgent_ExecuteRemote_Good(t *core.T) {
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

func TestAgent_ExecuteRemote_Bad(t *core.T) {
	ft := newFakeTransport() // no commands registered → "no match" error
	cfg := &AgentConfig{Transport: ft}
	a := NewAgent(cfg)

	if _, err := a.ExecuteRemote(context.Background(), "ls /"); err == nil {
		t.Error("expected error from fakeTransport with no registered patterns")
	}
}

func TestAgent_DiscoverCheckpoints_Ugly(t *core.T) {
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

func TestAgent_Influx_Good(t *core.T) {
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

// Spec §8 — ExecuteRemote(ctx, host, port, command) one-shot form.
// The 3-arg variant builds a transient SSHTransport rather than using the
// agent's configured transport. We cannot exercise the real SSH binary
// from a unit test, but we can confirm the argument-count routing is valid
// and that the 0-arg form is rejected cleanly.
func TestAgent_ExecuteRemote_Ugly(t *core.T) {
	cfg := &AgentConfig{Transport: newFakeTransport()}
	a := NewAgent(cfg)

	// Zero args — must error, not panic.
	if _, err := a.ExecuteRemote(context.Background()); err == nil {
		t.Error("expected error for 0-arg ExecuteRemote")
	}

	// Two args — neither 1-arg nor 3-arg form; must error.
	if _, err := a.ExecuteRemote(context.Background(), "host", "port"); err == nil {
		t.Error("expected error for 2-arg ExecuteRemote")
	}
}

// Spec §8 — Evaluate accepts Checkpoint, *Checkpoint or model-path string.
// With no transport registered, the nil-pointer path must surface a clean
// error rather than dereferencing.
func TestAgent_Evaluate_Bad(t *core.T) {
	cfg := &AgentConfig{Transport: newFakeTransport()}
	a := NewAgent(cfg)

	// Nil checkpoint pointer.
	if err := a.Evaluate(context.Background(), (*Checkpoint)(nil)); err == nil {
		t.Error("expected error for nil *Checkpoint")
	}

	// Unsupported target type.
	if err := a.Evaluate(context.Background(), 42); err == nil {
		t.Error("expected error for int target")
	}
}

func TestAgentResolveCheckpointTargetStringGoodScenario(t *core.T) {
	ft := newFakeTransport()
	base := "/data/training"

	ft.On("ls -d "+base+"/adapters-* 2>/dev/null",
		base+"/adapters-27b\n", nil)
	ft.On("ls -d "+base+"/adapters-27b/gemma-3-* 2>/dev/null", "", core.NewError("no match"))
	ft.On("ls "+base+"/adapters-27b/*_adapters.safetensors 2>/dev/null",
		base+"/adapters-27b/0001000_adapters.safetensors\n", nil)

	a := NewAgent(&AgentConfig{
		M3AdapterBase: base,
		Transport:     ft,
	})

	cp, err := a.resolveCheckpointTarget(context.Background(), base+"/adapters-27b")
	core.RequireNoError(t, err)
	core.AssertEqual(t, base+"/adapters-27b", cp.RemoteDir)
	core.AssertEqual(t, "adapters-27b", cp.Dirname)
	core.AssertEqual(t, "0001000_adapters.safetensors", cp.Filename)
	core.AssertEqual(t, "gemma-3-27b", cp.ModelTag)
	core.AssertNotEmpty(t, cp.Label)
	core.AssertNotEmpty(t, cp.RunID)
}

// Spec §8 — CollectMetrics accepts optional influxURL override.
func TestAgent_CollectMetrics_Good(t *core.T) {
	cfg := &AgentConfig{
		WorkDir:   t.TempDir(),
		InfluxURL: "http://default:8086",
		InfluxDB:  "test",
	}
	a := NewAgent(cfg)

	// Baseline — no override.
	if err := a.CollectMetrics(context.Background()); err != nil {
		t.Errorf("CollectMetrics baseline err = %v", err)
	}

	// Override URL.
	if err := a.CollectMetrics(context.Background(), "http://override:8086"); err != nil {
		t.Errorf("CollectMetrics override err = %v", err)
	}
}

// Spec §8 — Execute accepts optional config override.
func TestAgent_Execute_Good(t *core.T) {
	cfg := &AgentConfig{
		OneShot: true,
		WorkDir: t.TempDir(),
		// No real SSH host — the OneShot loop returns after discovery fails.
		M3AdapterBase: "/nonexistent",
		Transport:     newFakeTransport(),
	}
	a := NewAgent(cfg)

	// Default config path — completes promptly because OneShot is set.
	a.Execute(context.Background())

	// Override config — completes using supplied config.
	override := &AgentConfig{
		OneShot:       true,
		WorkDir:       t.TempDir(),
		M3AdapterBase: "/also-nonexistent",
		Transport:     newFakeTransport(),
	}
	a.Execute(context.Background(), override)
}

// ---------------------------------------------------------------------------
// Content probes — spec compatibility alias
// ---------------------------------------------------------------------------

type contentProbeBackend struct {
	prompts []string
}

func (b *contentProbeBackend) Generate(_ context.Context, prompt string, _ GenOpts) (Result, error) {
	b.prompts = append(b.prompts, prompt)
	return newResult("content", nil), nil
}

func (b *contentProbeBackend) Chat(_ context.Context, _ []Message, _ GenOpts) (Result, error) {
	return newResult("content", nil), nil
}

func (b *contentProbeBackend) Name() string    { return "content" }
func (b *contentProbeBackend) Available() bool { return true }

func TestRunContentProbesAlias_Good(t *core.T) {
	backend := &contentProbeBackend{}

	responses := RunContentProbes(context.Background(), backend)
	core.AssertLen(t, responses, len(ContentProbes))
	core.AssertLen(t, backend.prompts, len(ContentProbes))
	core.AssertEqual(t, ContentProbes[0].Prompt, backend.prompts[0])
	core.AssertEqual(t, ContentProbes[0].ID, responses[0].Probe.ID)
	core.AssertEqual(t, "content", responses[0].Response)
}

// ---------------------------------------------------------------------------
// GGUF — spec §7 wrappers
// ---------------------------------------------------------------------------

func TestGGUF_ReadGGUFInfo_Bad(t *core.T) {
	// Missing file must produce an error, not panic.
	if _, err := ReadGGUFInfo("/nonexistent/path/model.gguf"); err == nil {
		t.Error("expected error for missing GGUF file")
	}
}

func TestGGUF_DiscoverModels_Good(t *core.T) {
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

func TestGGUF_DiscoverModels_Ugly(t *core.T) {
	// Nonexistent path must not panic.
	path := "/definitely/not/a/real/path"
	models := DiscoverModels(path)
	core.AssertEqual(t, 0, len(models))
}
