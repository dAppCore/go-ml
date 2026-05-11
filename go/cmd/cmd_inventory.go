package cmd

import (
	"dappco.re/go"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
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

			result := ml.OpenDB(dbPath)
			if !result.OK {
				return resultFromError(coreerr.E("cmd.runInventory", "open db", errorFromResult(result)))
			}
			db := result.Value.(*ml.DB)
			defer db.Close()

			return resultFromError(ml.PrintInventory(db, nil))
		},
	})
}
