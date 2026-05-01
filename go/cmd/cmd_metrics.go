package cmd

import (
	"dappco.re/go"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
	"dappco.re/go/store"
)

// addMetricsCommand registers `ml metrics` — queries golden_set stats from
// DuckDB and pushes summary, per-domain, and per-voice metrics to InfluxDB.
//
//	core ml metrics --db lem.duckdb --influx http://10.69.69.165:8181
func addMetricsCommand(c *core.Core) {
	c.Command("ml/metrics", core.Command{
		Description: "Push golden set stats to InfluxDB",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			if dbPath == "" {
				return resultFromError(coreerr.E("cmd.runMetrics", "--db or LEM_DB required", nil))
			}

			db, err := store.OpenDuckDB(dbPath)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runMetrics", "open db", err))
			}
			defer db.Close()

			influx := ml.NewInfluxClient(influxURL, influxDB)

			return resultFromError(ml.PushMetrics(db, influx, nil))
		},
	})
}
