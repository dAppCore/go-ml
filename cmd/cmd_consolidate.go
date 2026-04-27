package cmd

import (
	"dappco.re/go/core"
	"dappco.re/go/ml"
)

// addConsolidateCommand registers `ml consolidate` — pulls JSONL responses
// from an M3 host over SSH/SCP, merges them by idx, dedupes, and writes a
// single merged JSONL output.
//
//	core ml consolidate --m3-host m3 --pattern "gold*.jsonl" --output ./responses
func addConsolidateCommand(c *core.Core) {
	c.Command("ml/consolidate", core.Command{
		Description: "Pull and merge response JSONL files from M3",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			cfg := ml.ConsolidateConfig{
				M3Host:    optStringOr(opts, "m3-host", "m3"),
				RemoteDir: optStringOr(opts, "remote", "/Volumes/Data/lem/responses"),
				Pattern:   optStringOr(opts, "pattern", "gold*.jsonl"),
				OutputDir: opts.String("output"),
				MergedOut: opts.String("merged"),
			}

			return resultFromError(ml.Consolidate(cfg, nil))
		},
	})
}
