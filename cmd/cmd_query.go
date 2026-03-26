package cmd

import (
	"io"
	"maps"
	"slices"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"forge.lthn.ai/core/cli/pkg/cli"
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

	db, err := ml.OpenDB(path)
	if err != nil {
		return coreerr.E("cmd.runQuery", "open db", err)
	}
	defer db.Close()

	sql := core.Join(" ", args...)

	// Auto-wrap non-SELECT queries as golden_set WHERE clauses.
	trimmed := core.Upper(core.Trim(sql))
	if !core.HasPrefix(trimmed, "SELECT") && !core.HasPrefix(trimmed, "SHOW") &&
		!core.HasPrefix(trimmed, "DESCRIBE") && !core.HasPrefix(trimmed, "EXPLAIN") {
		sql = "SELECT * FROM golden_set WHERE " + sql + " LIMIT 20"
	}

	rows, err := db.QueryRows(sql)
	if err != nil {
		return coreerr.E("cmd.runQuery", "query", err)
	}

	if queryJSON {
		if _, err := io.WriteString(cmd.OutOrStdout(), core.Concat(core.JSONMarshalString(rows), "\n")); err != nil {
			return coreerr.E("cmd.runQuery", "encode json", err)
		}
		core.Print(cmd.OutOrStdout(), "")
		core.Print(cmd.OutOrStdout(), "(%d rows)", len(rows))
		return nil
	}

	if len(rows) == 0 {
		core.Print(cmd.OutOrStdout(), "(0 rows)")
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
	w := cmd.OutOrStdout()
	for i, col := range cols {
		if i > 0 {
			io.WriteString(w, " | ")
		}
		io.WriteString(w, core.Sprintf("%-*s", widths[i], truncate(col, widths[i])))
	}
	io.WriteString(w, "\n")

	// Print separator.
	for i := range cols {
		if i > 0 {
			io.WriteString(w, "-+-")
		}
		io.WriteString(w, repeatString("-", widths[i]))
	}
	io.WriteString(w, "\n")

	// Print rows.
	for _, row := range rows {
		for i, col := range cols {
			if i > 0 {
				io.WriteString(w, " | ")
			}
			io.WriteString(w, core.Sprintf("%-*s", widths[i], truncate(formatValue(row[col]), widths[i])))
		}
		io.WriteString(w, "\n")
	}

	core.Print(w, "")
	core.Print(w, "(%d rows)", len(rows))
	return nil
}

func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
	return core.Sprint(v)
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

func repeatString(part string, count int) string {
	if count <= 0 {
		return ""
	}
	b := core.NewBuilder()
	for range count {
		b.WriteString(part)
	}
	return b.String()
}
