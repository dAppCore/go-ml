package cmd

import (
	"dappco.re/go"
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
				return core.Fail(core.E("cmd.runImportAll", "--db or LEM_DB required", nil))
			}

			dataDir := opts.String("data-dir")
			if dataDir == "" {
				dataDir = core.PathDir(dbPath)
			}

			db, result := store.OpenDuckDBReadWrite(dbPath)
			if !result.OK {
				return core.Fail(core.E("cmd.runImportAll", "open db", result.Value.(error)))
			}
			defer db.Close()

			cfg := store.ImportConfig{
				SkipM3:  opts.Bool("skip-m3"),
				DataDir: dataDir,
				M3Host:  optStringOr(opts, "m3-host", "m3"),
			}

			return store.ImportAll(db, cfg, nil)
		},
	})
}
