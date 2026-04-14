package cmd

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
)

// addInventoryCommand registers `ml inventory` — queries all DuckDB tables
// and prints row counts with per-table detail breakdowns.
//
//	core ml inventory --db lem.duckdb
func addInventoryCommand(c *core.Core) {
	c.Command("ml/inventory", core.Command{
		Description: "Show DuckDB table inventory with stats",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			if dbPath == "" {
				return resultFromError(coreerr.E("cmd.runInventory", "--db or LEM_DB required", nil))
			}

			db, err := ml.OpenDB(dbPath)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runInventory", "open db", err))
			}
			defer db.Close()

			return resultFromError(ml.PrintInventory(db, nil))
		},
	})
}
