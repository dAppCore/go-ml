package cmd

import (
	"dappco.re/go"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
	"dappco.re/go/store"
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

			db, result := store.OpenDuckDBReadWrite(dbPath)
			if !result.OK {
				return resultFromError(coreerr.E("cmd.runNormalize", "open db", errorFromResult(result)))
			}
			defer db.Close()

			cfg := ml.NormalizeConfig{
				MinLength: optInt(opts, "min-length", 50),
			}

			return resultFromError(ml.NormalizeSeeds(db, cfg, nil))
		},
	})
}
