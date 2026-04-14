package cmd

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"dappco.re/go/core/store"
)

// addApproveCommand registers `ml approve` — filter scored expansions above
// a threshold and export chat-format JSONL for training.
//
//	core ml approve --db /path/to/lem.duckdb --threshold 6.0 --output approved.jsonl
func addApproveCommand(c *core.Core) {
	c.Command("ml/approve", core.Command{
		Description: "Filter scored expansions into training JSONL",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			path := dbPath
			if path == "" {
				return resultFromError(coreerr.E("cmd.runApprove", "--db or LEM_DB required", nil))
			}

			output := opts.String("output")
			if output == "" {
				output = core.JoinPath(core.PathDir(path), "expansion-approved.jsonl")
			}
			threshold := optFloat(opts, "threshold", 6.0)

			db, err := store.OpenDuckDB(path)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runApprove", "open db", err))
			}
			defer db.Close()

			return resultFromError(ml.ApproveExpansions(db, ml.ApproveConfig{
				Output:    output,
				Threshold: threshold,
			}, nil))
		},
	})
}
