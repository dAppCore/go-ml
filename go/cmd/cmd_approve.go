package cmd

import (
	"dappco.re/go"
	"dappco.re/go/ml"
	"dappco.re/go/store"
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
				return core.Fail(core.E("cmd.runApprove", "--db or LEM_DB required", nil))
			}

			output := opts.String("output")
			if output == "" {
				output = core.JoinPath(core.PathDir(path), "expansion-approved.jsonl")
			}
			threshold := optFloat(opts, "threshold", 6.0)

			db, result := store.OpenDuckDB(path)
			if !result.OK {
				return core.Fail(core.E("cmd.runApprove", "open db", result.Value.(error)))
			}
			defer db.Close()

			return ml.ApproveExpansions(db, ml.ApproveConfig{
				Output:    output,
				Threshold: threshold,
			}, nil)
		},
	})
}
