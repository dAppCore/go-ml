package cmd

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
)

// addIngestCommand registers `ml ingest` — reads content score, capability
// score, and training log files and writes measurements to InfluxDB for the
// lab dashboard.
//
//	core ml ingest --content scores.jsonl --capability probe.jsonl --model gemma3:27b
func addIngestCommand(c *core.Core) {
	c.Command("ml/ingest", core.Command{
		Description: "Ingest benchmark scores and training logs into InfluxDB",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			if modelName == "" {
				return resultFromError(coreerr.E("cmd.runIngest", "--model is required", nil))
			}

			content := opts.String("content")
			capability := opts.String("capability")
			trainingLog := opts.String("training-log")
			if content == "" && capability == "" && trainingLog == "" {
				return resultFromError(coreerr.E("cmd.runIngest", "at least one of --content, --capability, or --training-log is required", nil))
			}

			influx := ml.NewInfluxClient(influxURL, influxDB)

			cfg := ml.IngestConfig{
				ContentFile:    content,
				CapabilityFile: capability,
				TrainingLog:    trainingLog,
				Model:          modelName,
				RunID:          opts.String("run-id"),
				BatchSize:      optInt(opts, "batch-size", 100),
			}

			return resultFromError(ml.Ingest(influx, cfg, nil))
		},
	})
}
