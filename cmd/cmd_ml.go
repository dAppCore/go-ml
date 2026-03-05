// Package cmd provides ML inference, scoring, and training pipeline commands.
package cmd

import (
	"forge.lthn.ai/core/cli/pkg/cli"
)

var mlCmd = cli.NewGroup("ml", "ML inference, scoring, and training pipeline",
	"Commands for ML model scoring, probe evaluation, data export, and format conversion.")

// AddMLCommands registers the 'ml' command and all subcommands.
func AddMLCommands(root *cli.Command) {
	initFlags()
	mlCmd.AddCommand(scoreCmd)
	mlCmd.AddCommand(probeCmd)
	mlCmd.AddCommand(exportCmd)
	mlCmd.AddCommand(expandCmd)
	mlCmd.AddCommand(statusCmd)
	mlCmd.AddCommand(ggufCmd)
	mlCmd.AddCommand(convertCmd)
	mlCmd.AddCommand(agentCmd)
	mlCmd.AddCommand(workerCmd)
	mlCmd.AddCommand(serveCmd)
	mlCmd.AddCommand(inventoryCmd)
	mlCmd.AddCommand(queryCmd)
	mlCmd.AddCommand(metricsCmd)
	mlCmd.AddCommand(ingestCmd)
	mlCmd.AddCommand(normalizeCmd)
	mlCmd.AddCommand(seedInfluxCmd)
	mlCmd.AddCommand(consolidateCmd)
	mlCmd.AddCommand(importCmd)
	mlCmd.AddCommand(approveCmd)
	mlCmd.AddCommand(publishCmd)
	mlCmd.AddCommand(coverageCmd)
	mlCmd.AddCommand(liveCmd)
	mlCmd.AddCommand(expandStatusCmd)
	root.AddCommand(mlCmd)
}

// Shared persistent flags.
var (
	apiURL     string
	judgeURL   string
	judgeModel string
	influxURL  string
	influxDB   string
	dbPath     string
	modelName  string
)

func initFlags() {
	cli.PersistentStringFlag(mlCmd, &apiURL, "api-url", "", "http://10.69.69.108:8090", "OpenAI-compatible API URL")
	cli.PersistentStringFlag(mlCmd, &judgeURL, "judge-url", "", "http://10.69.69.108:11434", "Judge model API URL (Ollama)")
	cli.PersistentStringFlag(mlCmd, &judgeModel, "judge-model", "", "gemma3:27b", "Judge model name")
	cli.PersistentStringFlag(mlCmd, &influxURL, "influx", "", "", "InfluxDB URL (default http://10.69.69.165:8181)")
	cli.PersistentStringFlag(mlCmd, &influxDB, "influx-db", "", "", "InfluxDB database (default training)")
	cli.PersistentStringFlag(mlCmd, &dbPath, "db", "", "", "DuckDB database path (or set LEM_DB env)")
	cli.PersistentStringFlag(mlCmd, &modelName, "model", "", "", "Model name for API")
}
