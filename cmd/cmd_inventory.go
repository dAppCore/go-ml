package cmd

import (
	"dappco.re/go/core"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/store"
	"dappco.re/go/core/cli/pkg/cli"
)

var inventoryCmd = &cli.Command{
	Use:   "inventory",
	Short: "Show DuckDB table inventory with stats",
	Long:  "Queries all DuckDB tables and prints row counts with per-table detail breakdowns.",
	RunE:  runInventory,
}

func runInventory(cmd *cli.Command, args []string) error {
	path := dbPath
	if path == "" {
		path = core.Env("LEM_DB")
	}
	if path == "" {
		return coreerr.E("cmd.runInventory", "--db or LEM_DB required", nil)
	}

	db, err := store.OpenDuckDB(path)
	if err != nil {
		return coreerr.E("cmd.runInventory", "open db", err)
	}
	defer db.Close()

<<<<<<< HEAD
	return store.PrintInventory(db, nil)
=======
	return ml.PrintInventory(db, cmd.OutOrStdout())
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
}
