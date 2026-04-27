package cmd

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
	"dappco.re/go/store"
)

// addSeedInfluxCommand registers `ml seed-influx` — one-time migration:
// batch-loads DuckDB golden_set records into InfluxDB golden_gen measurement.
//
//	core ml seed-influx --db lem.duckdb --batch-size 500
func addSeedInfluxCommand(c *core.Core) {
	c.Command("ml/seed-influx", core.Command{
		Description: "Seed InfluxDB golden_gen from DuckDB golden_set",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			if dbPath == "" {
				return resultFromError(coreerr.E("cmd.runSeedInflux", "--db or LEM_DB required", nil))
			}

			db, err := store.OpenDuckDB(dbPath)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runSeedInflux", "open db", err))
			}
			defer db.Close()

			influx := ml.NewInfluxClient(influxURL, influxDB)

			return resultFromError(ml.SeedInflux(db, influx, ml.SeedInfluxConfig{
				Force:     opts.Bool("force"),
				BatchSize: optInt(opts, "batch-size", 500),
			}, nil))
		},
	})
}
