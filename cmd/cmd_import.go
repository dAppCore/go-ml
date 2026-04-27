package cmd

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/log"
	"dappco.re/go/store"
)

// addImportCommand registers `ml import-all` — imports golden set, training
// examples, benchmark results, benchmark questions, and seeds into DuckDB
// from M3 and local files.
//
//	core ml import-all --db lem.duckdb --data-dir /Volumes/Data/lem
func addImportCommand(c *core.Core) {
	c.Command("ml/import-all", core.Command{
		Description: "Import all LEM data into DuckDB",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			if dbPath == "" {
				return resultFromError(coreerr.E("cmd.runImportAll", "--db or LEM_DB required", nil))
			}

			dataDir := opts.String("data-dir")
			if dataDir == "" {
				dataDir = core.PathDir(dbPath)
			}

			db, err := store.OpenDuckDBReadWrite(dbPath)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runImportAll", "open db", err))
			}
			defer db.Close()

			cfg := store.ImportConfig{
				SkipM3:  opts.Bool("skip-m3"),
				DataDir: dataDir,
				M3Host:  optStringOr(opts, "m3-host", "m3"),
			}

			return resultFromError(store.ImportAll(db, cfg, nil))
		},
	})
}
