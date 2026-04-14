// SPDX-License-Identifier: EUPL-1.2

package ml

import (
	"context"

	"dappco.re/go/core"
	corelog "dappco.re/go/core/log"
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
// or cfg.OneShot is set. Optional override config replaces the agent's
// stored config for this call only — useful for single-shot variants.
//
//	agent.Execute(ctx)
//	agent.Execute(ctx, overrideConfig)   // one-shot config override
func (a *Agent) Execute(ctx context.Context, override ...*AgentConfig) {
	cfg := a.cfg
	if len(override) > 0 && override[0] != nil {
		cfg = override[0]
	}
	RunAgentLoop(cfg)
	_ = ctx
}

// Evaluate scores a single checkpoint (adapter) and returns the probe
// results without the full discovery loop. The target argument accepts
// either a Checkpoint (direct struct) or a model path string that is
// resolved via DiscoverCheckpoints. Spec §8.
//
//	results, err := agent.Evaluate(ctx, ml.Checkpoint{...})
//	results, err := agent.Evaluate(ctx, "/models/adapter-42")
func (a *Agent) Evaluate(ctx context.Context, target any) error {
	influx := a.influxClient()
	switch v := target.(type) {
	case Checkpoint:
		return ProcessOne(a.cfg, influx, v)
	case *Checkpoint:
		if v == nil {
			return corelog.E("ml.Agent.Evaluate", "nil checkpoint", nil)
		}
		return ProcessOne(a.cfg, influx, *v)
	case string:
		cp := Checkpoint{
			RemoteDir: v,
			Dirname:   core.PathBase(v),
			Filename:  "adapters.safetensors",
		}
		return ProcessOne(a.cfg, influx, cp)
	default:
		return corelog.E("ml.Agent.Evaluate", core.Sprintf("unsupported target type %T", target), nil)
	}
}

// ExecuteRemote runs a shell command on the remote training host. The
// first positional arg MUST be the command; if host and port are passed
// in between ctx and command, a one-shot SSHTransport is built from them
// (useful when the caller does not want to rebuild the whole AgentConfig).
// Spec §8.
//
//	out, err := agent.ExecuteRemote(ctx, "ls /models")
//	out, err := agent.ExecuteRemote(ctx, "host.example", "2222", "uptime")
func (a *Agent) ExecuteRemote(ctx context.Context, args ...string) (string, error) {
	switch len(args) {
	case 0:
		return "", corelog.E("ml.Agent.ExecuteRemote", "no command supplied", nil)
	case 1:
		return a.cfg.transport().Run(ctx, args[0])
	case 3:
		host, port, command := args[0], args[1], args[2]
		keyPath := ""
		user := ""
		if a.cfg != nil {
			keyPath = a.cfg.M3SSHKey
			user = a.cfg.M3User
		}
		transport := NewSSHTransport(host, user, keyPath, WithPort(port))
		return transport.Run(ctx, command)
	default:
		return "", corelog.E("ml.Agent.ExecuteRemote",
			core.Sprintf("expected 1 arg (command) or 3 args (host,port,command); got %d", len(args)), nil)
	}
}

// CollectMetrics pushes queued probe/capability results to InfluxDB. Call
// this after Evaluate() has populated the internal buffer, or use it on a
// timer for long-running workflows. When influxURL is supplied, it
// replaces the agent's configured URL for this call only.
//
//	agent.CollectMetrics(ctx)
//	agent.CollectMetrics(ctx, "http://influx.local:8086")
func (a *Agent) CollectMetrics(ctx context.Context, influxURL ...string) error {
	influx := a.influxClient()
	if len(influxURL) > 0 && influxURL[0] != "" {
		influx = NewInfluxClient(influxURL[0], a.cfg.InfluxDB)
	}
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
