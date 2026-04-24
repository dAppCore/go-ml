package cmd

import (
	"dappco.re/go/core"
	"dappco.re/go/ml"
)

// addAgentCommand registers `ml agent` — polls M3 for unscored LoRA
// checkpoints, converts, probes, and pushes results to InfluxDB.
//
//	core ml agent --m3-host 10.69.69.108 --work-dir /tmp/scoring-agent
func addAgentCommand(c *core.Core) {
	c.Command("ml/agent", core.Command{
		Description: "Run the scoring agent daemon",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			cfg := &ml.AgentConfig{
				M3Host:        optStringOr(opts, "m3-host", ml.EnvOr("M3_HOST", "10.69.69.108")),
				M3User:        optStringOr(opts, "m3-user", ml.EnvOr("M3_USER", "claude")),
				M3SSHKey:      optStringOr(opts, "m3-ssh-key", ml.EnvOr("M3_SSH_KEY", ml.ExpandHome("~/.ssh/id_ed25519"))),
				M3AdapterBase: optStringOr(opts, "m3-adapter-base", ml.EnvOr("M3_ADAPTER_BASE", "/Volumes/Data/lem")),
				InfluxURL:     influxURL,
				InfluxDB:      influxDB,
				DBPath:        dbPath,
				APIURL:        apiURL,
				JudgeURL:      judgeURL,
				JudgeModel:    judgeModel,
				Model:         modelName,
				BaseModel:     optStringOr(opts, "base-model", ml.EnvOr("BASE_MODEL", "deepseek-ai/DeepSeek-R1-Distill-Qwen-7B")),
				PollInterval:  optInt(opts, "poll", ml.IntEnvOr("POLL_INTERVAL", 300)),
				WorkDir:       optStringOr(opts, "work-dir", ml.EnvOr("WORK_DIR", "/tmp/scoring-agent")),
				Filter:        opts.String("filter"),
				Force:         optBool(opts, "force"),
				OneShot:       optBool(opts, "one-shot"),
				DryRun:        optBool(opts, "dry-run"),
			}

			ml.RunAgentLoop(cfg)
			return core.Result{OK: true}
		},
	})
}
