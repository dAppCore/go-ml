package cmd

import (
	"dappco.re/go/core"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"forge.lthn.ai/core/cli/pkg/cli"
)

var seedInfluxCmd = &cli.Command{
	Use:   "seed-influx",
	Short: "Seed InfluxDB golden_gen from DuckDB golden_set",
	Long:  "One-time migration: batch-loads DuckDB golden_set records into InfluxDB golden_gen measurement.",
	RunE:  runSeedInflux,
}

var (
	seedInfluxForce     bool
	seedInfluxBatchSize int
)

func init() {
	seedInfluxCmd.Flags().BoolVar(&seedInfluxForce, "force", false, "Re-seed even if InfluxDB already has data")
	seedInfluxCmd.Flags().IntVar(&seedInfluxBatchSize, "batch-size", 500, "Lines per InfluxDB write batch")
}

func runSeedInflux(cmd *cli.Command, args []string) error {
	path := dbPath
	if path == "" {
		path = core.Env("LEM_DB")
	}
	if path == "" {
		return coreerr.E("cmd.runSeedInflux", "--db or LEM_DB required", nil)
	}

	db, err := ml.OpenDB(path)
	if err != nil {
		return coreerr.E("cmd.runSeedInflux", "open db", err)
	}
	defer db.Close()

	influx := ml.NewInfluxClient(influxURL, influxDB)

	return ml.SeedInflux(db, influx, ml.SeedInfluxConfig{
		Force:     seedInfluxForce,
		BatchSize: seedInfluxBatchSize,
	}, cmd.OutOrStdout())
}
