package cmd

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"dappco.re/go/core/store"
)

// addNormalizeCommand registers `ml normalize` — deduplicates seeds against
// golden_set and prompts, creating the expansion_prompts table with
// priority-based ordering.
//
//	core ml normalize --db lem.duckdb --min-length 50
func addNormalizeCommand(c *core.Core) {
	c.Command("ml/normalize", core.Command{
		Description: "Normalize seeds into expansion prompts",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			if dbPath == "" {
				return resultFromError(coreerr.E("cmd.runNormalize", "--db or LEM_DB env is required", nil))
			}

			db, err := store.OpenDuckDBReadWrite(dbPath)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runNormalize", "open db", err))
			}
			defer db.Close()

			cfg := ml.NormalizeConfig{
				MinLength: optInt(opts, "min-length", 50),
			}

			return resultFromError(ml.NormalizeSeeds(db, cfg, nil))
		},
	})
}
