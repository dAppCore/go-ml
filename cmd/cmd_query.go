package cmd

import (
	"dappco.re/go/core"
	"encoding/json"
	"maps"
	"slices"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/store"
	"dappco.re/go/core/cli/pkg/cli"
)

var queryCmd = &cli.Command{
	Use:   "query [sql]",
	Short: "Run ad-hoc SQL against DuckDB",
	Long:  "Executes arbitrary SQL against the DuckDB database. Non-SELECT queries are auto-wrapped as golden_set WHERE clauses.",
	Example: `  core ml query "SELECT COUNT(*) FROM golden_set"
  core ml query "domain = 'ethics'"
  core ml query --json "SHOW TABLES"`,
	Args: cli.MinimumNArgs(1),
	RunE: runQuery,
}

var queryJSON bool

func init() {
	queryCmd.Flags().BoolVar(&queryJSON, "json", false, "Output as JSON")
}

func runQuery(cmd *cli.Command, args []string) error {
	path := dbPath
	if path == "" {
		path = core.Env("LEM_DB")
	}
	if path == "" {
		return coreerr.E("cmd.runQuery", "--db or LEM_DB env is required", nil)
	}

	db, err := store.OpenDuckDB(path)
	if err != nil {
		return coreerr.E("cmd.runQuery", "open db", err)
	}
	defer db.Close()

	sql := joinStrings(args, " ")

	// Auto-wrap non-SELECT queries as golden_set WHERE clauses.
	trimmed := core.Trim(core.Upper(sql))
	if !core.HasPrefix(trimmed, "SELECT") && !core.HasPrefix(trimmed, "SHOW") &&
		!core.HasPrefix(trimmed, "DESCRIBE") && !core.HasPrefix(trimmed, "EXPLAIN") {
		sql = "SELECT * FROM golden_set WHERE " + sql + " LIMIT 20"
	}

	rows, err := db.QueryRows(sql)
	if err != nil {
		return coreerr.E("cmd.runQuery", "query", err)
	}

	if queryJSON {
		enc := json.NewEncoder(nil)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			return coreerr.E("cmd.runQuery", "encode json", err)
		}
		core.Print(nil, "\n(%d rows)\n", len(rows))
		return nil
	}

	if len(rows) == 0 {
		core.Println("(0 rows)")
		return nil
	}

	// Collect column names in stable order from first row.
	cols := slices.Sorted(maps.Keys(rows[0]))

	// Calculate column widths (capped at 60).
	const maxWidth = 60
	widths := make([]int, len(cols))
	for i, col := range cols {
		widths[i] = len(col)
	}
	for _, row := range rows {
		for i, col := range cols {
			val := formatValue(row[col])
			if l := len(val); l > widths[i] {
				widths[i] = l
			}
		}
	}
	for i := range widths {
		if widths[i] > maxWidth {
			widths[i] = maxWidth
		}
	}

	// Print header.
	for i, col := range cols {
		if i > 0 {
			printf(" | ")
		}
		core.Print(nil,("%-*s", widths[i], truncate(col, widths[i]))
	}
	core.Println()

	// Print separator.
	for i := range cols {
		if i > 0 {
			printf("-+-")
		}
		printf(repeatStr("-", widths[i]))
	}
	core.Println()

	// Print rows.
	for _, row := range rows {
		for i, col := range cols {
			if i > 0 {
				printf(" | ")
			}
			core.Print(nil,("%-*s", widths[i], truncate(formatValue(row[col]), widths[i]))
		}
		core.Println()
	}

	core.Print(nil,("\n(%d rows)\n", len(rows))
	return nil
}

func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
	return core.Sprintf("%v", v)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
