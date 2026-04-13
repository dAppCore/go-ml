package cmd

import (
	"dappco.re/go/core"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"dappco.re/go/core/store"
	"dappco.re/go/core/cli/pkg/cli"
)

var statusCmd = &cli.Command{
	Use:   "status",
	Short: "Show training and generation progress",
	Long:  "Queries InfluxDB for training status, loss, and generation progress. Optionally shows DuckDB table counts.",
	RunE:  runStatus,
}

func runStatus(cmd *cli.Command, args []string) error {
	influx := ml.NewInfluxClient(influxURL, influxDB)

	if err := ml.PrintStatus(influx, nil); err != nil {
		return coreerr.E("cmd.runStatus", "status", err)
	}

	path := dbPath
	if path == "" {
		path = core.Env("LEM_DB")
	}

	if path != "" {
		db, err := store.OpenDuckDB(path)
		if err != nil {
			return coreerr.E("cmd.runStatus", "open db", err)
		}
		defer db.Close()

		counts, err := db.TableCounts()
		if err != nil {
			return coreerr.E("cmd.runStatus", "table counts", err)
		}

		core.Println()
		core.Println("DuckDB:")
		order := []string{"golden_set", "expansion_prompts", "seeds", "training_examples",
			"prompts", "gemini_responses", "benchmark_questions", "benchmark_results", "validations"}
		for _, table := range order {
			if count, ok := counts[table]; ok {
				core.Print(nil, "  %-22s %6d rows\n", table, count)
			}
		}
	}

	return nil
}
