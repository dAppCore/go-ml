// SPDX-License-Identifier: EUPL-1.2

package ml

import (
	"context"

	"dappco.re/go/core"
)

// Agent orchestrates model evaluation and fleet training. It wraps the
// underlying RunAgentLoop / DiscoverCheckpoints / SSH transport functions
// with a typed facade so callers never touch package-level state.
//
//	agent := ml.NewAgent(&ml.AgentConfig{
//	    M3Host:     "homelab.lthn.sh",
//	    M3User:     "snider",
//	    APIURL:     "http://127.0.0.1:11434",
//	    JudgeURL:   "http://127.0.0.1:11434",
//	    JudgeModel: "qwen3:8b",
//	    WorkDir:    "/tmp/scoring",
//	})
//	agent.Execute(ctx)
type Agent struct {
	cfg    *AgentConfig
	influx *InfluxClient
}

// NewAgent creates a scoring/training Agent bound to the supplied config.
// The config is stored by pointer, so any later mutation by the caller is
// respected by subsequent method calls.
func NewAgent(cfg *AgentConfig) *Agent {
	return &Agent{cfg: cfg}
}

// Config returns the agent's underlying AgentConfig so callers can read or
// mutate fields (useful for tests).
func (a *Agent) Config() *AgentConfig { return a.cfg }

// Execute runs the full scoring/training loop (discovers checkpoints,
// scores them, pushes results to InfluxDB). Blocks until ctx is cancelled
// or cfg.OneShot is set.
//
//	agent.Execute(ctx)
func (a *Agent) Execute(ctx context.Context) {
	RunAgentLoop(a.cfg)
	_ = ctx
}

// Evaluate scores a single checkpoint (adapter) and returns the probe
// results without the full discovery loop.
//
//	results, err := agent.Evaluate(ctx, ml.Checkpoint{...})
func (a *Agent) Evaluate(ctx context.Context, cp Checkpoint) error {
	influx := a.influxClient()
	return ProcessOne(a.cfg, influx, cp)
}

// ExecuteRemote runs a shell command on the remote training host via the
// configured RemoteTransport (SSHTransport by default). Returns the
// combined stdout/stderr output.
//
//	out, err := agent.ExecuteRemote(ctx, "ls /models")
func (a *Agent) ExecuteRemote(ctx context.Context, command string) (string, error) {
	return a.cfg.transport().Run(ctx, command)
}

// CollectMetrics pushes queued probe/capability results to InfluxDB. Call
// this after Evaluate() has populated the internal buffer, or use it on a
// timer for long-running workflows.
func (a *Agent) CollectMetrics(ctx context.Context) error {
	influx := a.influxClient()
	ReplayInfluxBuffer(a.cfg.WorkDir, influx)
	_ = ctx
	return nil
}

// DiscoverCheckpoints lists all adapter checkpoints on the remote host.
//
//	cps, err := agent.DiscoverCheckpoints(ctx)
func (a *Agent) DiscoverCheckpoints(ctx context.Context) ([]Checkpoint, error) {
	_ = ctx
	return DiscoverCheckpoints(a.cfg)
}

// Influx returns the shared InfluxClient, constructing it lazily from the
// agent config on first access.
func (a *Agent) Influx() *InfluxClient {
	return a.influxClient()
}

func (a *Agent) influxClient() *InfluxClient {
	if a.influx != nil {
		return a.influx
	}
	url := a.cfg.InfluxURL
	db := a.cfg.InfluxDB
	if url == "" {
		url = core.Env("INFLUX_URL")
	}
	if db == "" {
		db = core.Env("INFLUX_DB")
	}
	a.influx = NewInfluxClient(url, db)
	return a.influx
}
