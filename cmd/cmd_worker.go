package cmd

import (
	"time"

	"dappco.re/go/core"
	"dappco.re/go/core/ml"
)

// addWorkerCommand registers `ml worker` — polls the LEM API for tasks, runs
// local inference, and submits results.
//
//	core ml worker --api https://infer.lthn.ai --key $LEM_API_KEY --infer http://localhost:8090
func addWorkerCommand(c *core.Core) {
	c.Command("ml/worker", core.Command{
		Description: "Run a distributed worker node",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			apiKey := optStringOr(opts, "key", ml.EnvOr("LEM_API_KEY", ""))
			if apiKey == "" {
				apiKey = ml.ReadKeyFile()
			}

			pollSec := optInt(opts, "poll", 30)

			cfg := &ml.WorkerConfig{
				APIBase:      optStringOr(opts, "api", ml.EnvOr("LEM_API", "https://infer.lthn.ai")),
				WorkerID:     optStringOr(opts, "id", ml.EnvOr("LEM_WORKER_ID", ml.MachineID())),
				Name:         optStringOr(opts, "name", ml.EnvOr("LEM_WORKER_NAME", ml.Hostname())),
				APIKey:       apiKey,
				GPUType:      optStringOr(opts, "gpu", ml.EnvOr("LEM_GPU", "")),
				VRAMGb:       optInt(opts, "vram", ml.IntEnvOr("LEM_VRAM_GB", 0)),
				InferURL:     optStringOr(opts, "infer", ml.EnvOr("LEM_INFER_URL", "http://localhost:8090")),
				TaskType:     opts.String("type"),
				BatchSize:    optInt(opts, "batch", 5),
				PollInterval: time.Duration(pollSec) * time.Second,
				OneShot:      opts.Bool("one-shot"),
				DryRun:       opts.Bool("dry-run"),
			}

			if langs := optStringOr(opts, "languages", ml.EnvOr("LEM_LANGUAGES", "")); langs != "" {
				cfg.Languages = ml.SplitComma(langs)
			}
			if models := optStringOr(opts, "models", ml.EnvOr("LEM_MODELS", "")); models != "" {
				cfg.Models = ml.SplitComma(models)
			}

			ml.RunWorkerLoop(cfg)
			return core.Result{OK: true}
		},
	})
}
