package cmd

import (
	"dappco.re/go"
	"dappco.re/go/ml"
	"dappco.re/go/store"
)

// addCoverageCommand registers `ml coverage` — queries seeds by region and
// domain, renders ASCII bar charts, and highlights underrepresented areas.
//
//	core ml coverage --db /Volumes/Data/lem/lem.duckdb
func addCoverageCommand(c *core.Core) {
	c.Command("ml/coverage", core.Command{
		Description: "Analyze seed coverage by region and domain",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			if dbPath == "" {
				return core.Fail(core.E("cmd.runCoverage", "--db or LEM_DB required", nil))
			}

			db, result := store.OpenDuckDB(dbPath)
			if !result.OK {
				return core.Fail(core.E("cmd.runCoverage", "open db", result.Value.(error)))
			}
			defer db.Close()

			return ml.PrintCoverage(db, nil)
		},
	})
}
