package cmd

import (
	"dappco.re/go"
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
				return core.Fail(core.E("cmd.runInventory", "--db or LEM_DB required", nil))
			}

			result := ml.OpenDB(dbPath)
			if !result.OK {
				return core.Fail(core.E("cmd.runInventory", "open db", result.Value.(error)))
			}
			db := result.Value.(*ml.DB)
			defer db.Close()

			return ml.PrintInventory(db, nil)
		},
	})
}
