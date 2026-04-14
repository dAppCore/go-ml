package cmd

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/store"
)

// addExpandStatusCommand registers `ml expand-status` — queries DuckDB for
// expansion prompts, generated responses, scoring status, and overall pipeline
// progress.
//
//	core ml expand-status --db /Volumes/Data/lem/lem.duckdb
func addExpandStatusCommand(c *core.Core) {
	c.Command("ml/expand-status", core.Command{
		Description: "Show expansion pipeline progress",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			if dbPath == "" {
				return resultFromError(coreerr.E("cmd.runExpandStatus", "--db or LEM_DB required", nil))
			}

			db, err := store.OpenDuckDB(dbPath)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runExpandStatus", "open db", err))
			}
			defer db.Close()

			core.Print(nil, "LEM Expansion Pipeline Status")
			core.Print(nil, "==================================================")

			// Expansion prompts
			total, pending, err := db.CountExpansionPrompts()
			if err != nil {
				core.Print(nil, "  Expansion prompts:  not created (run: normalize)")
				return core.Result{OK: true}
			}
			core.Print(nil, "  Expansion prompts:  %d total, %d pending", total, pending)

			// Generated responses — query raw counts via SQL
			generated := 0
			rows, err := db.QueryRows("SELECT count(*) AS n FROM expansion_raw")
			if err != nil || len(rows) == 0 {
				core.Print(nil, "  Generated:          0 (run: core ml expand)")
			} else {
				if n, ok := rows[0]["n"]; ok {
					generated = toInt(n)
				}
				core.Print(nil, "  Generated:          %d", generated)
			}

			// Scored — query scoring counts via SQL
			sRows, err := db.QueryRows("SELECT count(*) AS n FROM scoring_results WHERE suite = 'heuristic'")
			if err != nil || len(sRows) == 0 {
				core.Print(nil, "  Scored:             0 (run: score --tier 1)")
			} else {
				scored := toInt(sRows[0]["n"])
				core.Print(nil, "  Heuristic scored:   %d", scored)
			}

			// Pipeline progress
			if total > 0 && generated > 0 {
				genPct := float64(generated) / float64(total) * 100
				core.Print(nil, "")
				core.Print(nil, "  Progress:           %.1f%% generated", genPct)
			}

			// Golden set context
			golden, err := db.CountGoldenSet()
			if err == nil && golden > 0 {
				core.Print(nil, "")
				core.Print(nil, "  Golden set:         %d / %d", golden, targetTotal)
				if generated > 0 {
					core.Print(nil, "  Combined:           %d total examples", golden+generated)
				}
			}

			return core.Result{OK: true}
		},
	})
}

// toInt converts an any (typically from QueryRows) to int.
//
//	total := toInt(rows[0]["n"])
func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}
