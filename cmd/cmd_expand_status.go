package cmd

import (
	"fmt"
	"os"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"forge.lthn.ai/core/cli/pkg/cli"
)

var expandStatusCmd = &cli.Command{
	Use:   "expand-status",
	Short: "Show expansion pipeline progress",
	Long:  "Queries DuckDB for expansion prompts, generated responses, scoring status, and overall pipeline progress.",
	RunE:  runExpandStatus,
}

func runExpandStatus(cmd *cli.Command, args []string) error {
	path := dbPath
	if path == "" {
		path = os.Getenv("LEM_DB")
	}
	if path == "" {
		return coreerr.E("cmd.runExpandStatus", "--db or LEM_DB required", nil)
	}

	db, err := ml.OpenDB(path)
	if err != nil {
		return coreerr.E("cmd.runExpandStatus", "open db", err)
	}
	defer db.Close()

	fmt.Fprintln(os.Stdout, "LEM Expansion Pipeline Status")
	fmt.Fprintln(os.Stdout, "==================================================")

	// Expansion prompts
	total, pending, err := db.CountExpansionPrompts()
	if err != nil {
		fmt.Fprintln(os.Stdout, "  Expansion prompts:  not created (run: normalize)")
		return nil
	}
	fmt.Fprintf(os.Stdout, "  Expansion prompts:  %d total, %d pending\n", total, pending)

	// Generated responses — query raw counts via SQL
	generated := 0
	rows, err := db.QueryRows("SELECT count(*) AS n FROM expansion_raw")
	if err != nil || len(rows) == 0 {
		fmt.Fprintln(os.Stdout, "  Generated:          0 (run: core ml expand)")
	} else {
		if n, ok := rows[0]["n"]; ok {
			generated = toInt(n)
		}
		fmt.Fprintf(os.Stdout, "  Generated:          %d\n", generated)
	}

	// Scored — query scoring counts via SQL
	sRows, err := db.QueryRows("SELECT count(*) AS n FROM scoring_results WHERE suite = 'heuristic'")
	if err != nil || len(sRows) == 0 {
		fmt.Fprintln(os.Stdout, "  Scored:             0 (run: score --tier 1)")
	} else {
		scored := toInt(sRows[0]["n"])
		fmt.Fprintf(os.Stdout, "  Heuristic scored:   %d\n", scored)
	}

	// Pipeline progress
	if total > 0 && generated > 0 {
		genPct := float64(generated) / float64(total) * 100
		fmt.Fprintf(os.Stdout, "\n  Progress:           %.1f%% generated\n", genPct)
	}

	// Golden set context
	golden, err := db.CountGoldenSet()
	if err == nil && golden > 0 {
		fmt.Fprintf(os.Stdout, "\n  Golden set:         %d / %d\n", golden, targetTotal)
		if generated > 0 {
			fmt.Fprintf(os.Stdout, "  Combined:           %d total examples\n", golden+generated)
		}
	}

	return nil
}

// toInt converts an any (typically from QueryRows) to int.
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
