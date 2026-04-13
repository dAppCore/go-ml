package cmd

import (
<<<<<<< HEAD
	"dappco.re/go/core"
	"encoding/json"
=======
	"io"
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	"maps"
	"slices"

	"dappco.re/go/core"
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

<<<<<<< HEAD
	sql := joinStrings(args, " ")

	// Auto-wrap non-SELECT queries as golden_set WHERE clauses.
	trimmed := core.Trim(core.Upper(sql))
=======
	sql := core.Join(" ", args...)

	// Auto-wrap non-SELECT queries as golden_set WHERE clauses.
	trimmed := core.Upper(core.Trim(sql))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	if !core.HasPrefix(trimmed, "SELECT") && !core.HasPrefix(trimmed, "SHOW") &&
		!core.HasPrefix(trimmed, "DESCRIBE") && !core.HasPrefix(trimmed, "EXPLAIN") {
		sql = "SELECT * FROM golden_set WHERE " + sql + " LIMIT 20"
	}

	rows, err := db.QueryRows(sql)
	if err != nil {
		return coreerr.E("cmd.runQuery", "query", err)
	}

	if queryJSON {
<<<<<<< HEAD
		enc := json.NewEncoder(nil)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			return coreerr.E("cmd.runQuery", "encode json", err)
		}
		core.Print(nil, "\n(%d rows)\n", len(rows))
=======
		if _, err := io.WriteString(cmd.OutOrStdout(), core.Concat(core.JSONMarshalString(rows), "\n")); err != nil {
			return coreerr.E("cmd.runQuery", "encode json", err)
		}
		core.Print(cmd.OutOrStdout(), "")
		core.Print(cmd.OutOrStdout(), "(%d rows)", len(rows))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
		return nil
	}

	if len(rows) == 0 {
<<<<<<< HEAD
		core.Println("(0 rows)")
=======
		core.Print(cmd.OutOrStdout(), "(0 rows)")
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
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
<<<<<<< HEAD
			printf(" | ")
		}
		core.Print(nil,("%-*s", widths[i], truncate(col, widths[i]))
	}
	core.Println()
=======
			io.WriteString(w, " | ")
		}
		io.WriteString(w, core.Sprintf("%-*s", widths[i], truncate(col, widths[i])))
	}
	io.WriteString(w, "\n")
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

	// Print separator.
	for i := range cols {
		if i > 0 {
<<<<<<< HEAD
			printf("-+-")
		}
		printf(repeatStr("-", widths[i]))
	}
	core.Println()
=======
			io.WriteString(w, "-+-")
		}
		io.WriteString(w, repeatString("-", widths[i]))
	}
	io.WriteString(w, "\n")
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

	// Print rows.
	for _, row := range rows {
		for i, col := range cols {
			if i > 0 {
<<<<<<< HEAD
				printf(" | ")
			}
			core.Print(nil,("%-*s", widths[i], truncate(formatValue(row[col]), widths[i]))
		}
		core.Println()
	}

	core.Print(nil,("\n(%d rows)\n", len(rows))
=======
				io.WriteString(w, " | ")
			}
			io.WriteString(w, core.Sprintf("%-*s", widths[i], truncate(formatValue(row[col]), widths[i])))
		}
		io.WriteString(w, "\n")
	}

	core.Print(w, "")
	core.Print(w, "(%d rows)", len(rows))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	return nil
}

func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
<<<<<<< HEAD
	return core.Sprintf("%v", v)
=======
	return core.Sprint(v)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
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
