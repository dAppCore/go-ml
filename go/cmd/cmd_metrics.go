package cmd

import (
	"dappco.re/go"
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
				return core.Fail(core.E("cmd.runMetrics", "--db or LEM_DB required", nil))
			}

			db, result := store.OpenDuckDB(dbPath)
			if !result.OK {
				return core.Fail(core.E("cmd.runMetrics", "open db", result.Value.(error)))
			}
			defer db.Close()

			influx := ml.NewInfluxClient(influxURL, influxDB)

			return ml.PushMetrics(db, influx, nil)
		},
	})
}
