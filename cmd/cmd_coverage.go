package cmd

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/log"
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
				return resultFromError(coreerr.E("cmd.runCoverage", "--db or LEM_DB required", nil))
			}

			db, err := store.OpenDuckDB(dbPath)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runCoverage", "open db", err))
			}
			defer db.Close()

			return resultFromError(ml.PrintCoverage(db, nil))
		},
	})
}
