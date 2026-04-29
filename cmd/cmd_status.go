package cmd

import (
	"dappco.re/go"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
	"dappco.re/go/store"
)

// addStatusCommand registers `ml status` — queries InfluxDB for training
// status, loss, and generation progress. Optionally shows DuckDB table counts.
//
//	core ml status --db lem.duckdb --influx http://10.69.69.165:8181
func addStatusCommand(c *core.Core) {
	c.Command("ml/status", core.Command{
		Description: "Show training and generation progress",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			influx := ml.NewInfluxClient(influxURL, influxDB)

			if err := ml.PrintStatus(influx, nil); err != nil {
				return resultFromError(coreerr.E("cmd.runStatus", "status", err))
			}

			if dbPath != "" {
				db, err := store.OpenDuckDB(dbPath)
				if err != nil {
					return resultFromError(coreerr.E("cmd.runStatus", "open db", err))
				}
				defer db.Close()

				counts, err := db.TableCounts()
				if err != nil {
					return resultFromError(coreerr.E("cmd.runStatus", "table counts", err))
				}

				core.Print(nil, "")
				core.Print(nil, "DuckDB:")
				order := []string{"golden_set", "expansion_prompts", "seeds", "training_examples",
					"prompts", "gemini_responses", "benchmark_questions", "benchmark_results", "validations"}
				for _, table := range order {
					if count, ok := counts[table]; ok {
						core.Print(nil, "  %-22s %6d rows", table, count)
					}
				}
			}

			return core.Result{OK: true}
		},
	})
}
