// Package cmd provides ML inference, scoring, and training pipeline commands.
//
// Commands are registered via AddMLCommands against a *core.Core instance,
// producing a tree under the "ml" path. The go-cli runtime mounts them.
package cmd

import (
	"strconv"

	"dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
	"dappco.re/go/ml"
)

func init() {
	cli.RegisterCommands(AddMLCommands)
}

// Shared persistent flags — populated lazily by each subcommand's Action
// via opts.String("..."). Kept as package-level so helper code can read
// them after parsing.
var (
	apiURL     string
	judgeURL   string
	judgeModel string
	influxURL  string
	influxDB   string
	dbPath     string
	modelName  string
)

// AddMLCommands registers the 'ml' parent command and all subcommands on
// the given Core instance.
//
//	cli.RegisterCommands(cmd.AddMLCommands)
func AddMLCommands(c *core.Core) {
	c.Command("ml", core.Command{
		Description: "ML inference, scoring, and training pipeline",
	})

	addApproveCommand(c)
	addAgentCommand(c)
	addConsolidateCommand(c)
	addConvertCommand(c)
	addCoverageCommand(c)
	addEvaluateCommand(c)
	addExpandCommand(c)
	addExpandStatusCommand(c)
	addExportCommand(c)
	addGGUFCommand(c)
	addImportCommand(c)
	addIngestCommand(c)
	addInventoryCommand(c)
	addLiveCommand(c)
	addMetricsCommand(c)
	addNormalizeCommand(c)
	addProbeCommand(c)
	addPublishCommand(c)
	addQueryCommand(c)
	addScoreCommand(c)
	addSeedInfluxCommand(c)
	addServeCommand(c)
	addStatusCommand(c)
	addWorkerCommand(c)
}

// readPersistentFlags populates the shared persistent-flag variables from
// opts — every subcommand invokes it to give the package state its values
// before running the action body. Persistent flags default to env-driven
// values when unset.
func readPersistentFlags(opts core.Options) {
	apiURL = optStringOr(opts, "api-url", ml.EnvOr("ML_API_URL", "http://10.69.69.108:8090"))
	judgeURL = optStringOr(opts, "judge-url", ml.EnvOr("ML_JUDGE_URL", "http://10.69.69.108:11434"))
	judgeModel = optStringOr(opts, "judge-model", ml.EnvOr("ML_JUDGE_MODEL", "gemma3:27b"))
	influxURL = optStringOr(opts, "influx", ml.EnvOr("ML_INFLUX_URL", ""))
	influxDB = optStringOr(opts, "influx-db", ml.EnvOr("ML_INFLUX_DB", ""))
	dbPath = optStringOr(opts, "db", ml.EnvOr("LEM_DB", ""))
	modelName = optStringOr(opts, "model", ml.EnvOr("ML_MODEL", ""))
}

// optStringOr returns opts[key] if set, else fallback.
//
//	name := optStringOr(opts, "model", "gemma3:27b")
func optStringOr(opts core.Options, key, fallback string) string {
	if v := opts.String(key); v != "" {
		return v
	}
	return fallback
}

// optBool returns opts[key] as bool.
//
//	verbose := optBool(opts, "verbose")
func optBool(opts core.Options, key string) bool {
	return opts.Bool(key)
}

// optInt returns opts[key] as int, falling back to fallback when zero/missing.
//
//	port := optInt(opts, "port", 8080)
func optInt(opts core.Options, key string, fallback int) int {
	if v := opts.Int(key); v != 0 {
		return v
	}
	return fallback
}

// optFloat returns opts[key] as float64 via string parse, falling back when missing.
//
//	threshold := optFloat(opts, "threshold", 6.0)
func optFloat(opts core.Options, key string, fallback float64) float64 {
	s := opts.String(key)
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fallback
	}
	return v
}

// resultFromError packages a Go error into a core.Result — OK on nil,
// Value=err + OK=false otherwise. Used by every command Action.
//
//	return resultFromError(run(ctx))
func resultFromError(err error) core.Result {
	if err != nil {
		return core.Result{Value: err, OK: false}
	}
	return core.Result{OK: true}
}
