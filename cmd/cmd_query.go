package cmd

import (
	"maps"
	"slices"

	"dappco.re/go/core"
	coreerr "dappco.re/go/log"
	"dappco.re/go/store"
)

// addQueryCommand registers `ml query` — executes arbitrary SQL against the
// DuckDB database. Non-SELECT queries are auto-wrapped as golden_set WHERE
// clauses.
//
//	core ml query "SELECT COUNT(*) FROM golden_set"
//	core ml query "domain = 'ethics'"
//	core ml query --json "SHOW TABLES"
func addQueryCommand(c *core.Core) {
	c.Command("ml/query", core.Command{
		Description: "Run ad-hoc SQL against DuckDB",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			if dbPath == "" {
				return resultFromError(coreerr.E("cmd.runQuery", "--db or LEM_DB env is required", nil))
			}

			sql := opts.String("_arg")
			if sql == "" {
				return resultFromError(coreerr.E("cmd.runQuery", "SQL argument required", nil))
			}

			db, err := store.OpenDuckDB(dbPath)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runQuery", "open db", err))
			}
			defer db.Close()

			// Auto-wrap non-SELECT queries as golden_set WHERE clauses.
			trimmed := core.Upper(core.Trim(sql))
			if !core.HasPrefix(trimmed, "SELECT") && !core.HasPrefix(trimmed, "SHOW") &&
				!core.HasPrefix(trimmed, "DESCRIBE") && !core.HasPrefix(trimmed, "EXPLAIN") {
				sql = "SELECT * FROM golden_set WHERE " + sql + " LIMIT 20"
			}

			rows, err := db.QueryRows(sql)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runQuery", "query", err))
			}

			jsonMode := opts.Bool("json")
			if jsonMode {
				core.Print(nil, "%s", core.JSONMarshalString(rows))
				core.Print(nil, "(%d rows)", len(rows))
				return core.Result{OK: true}
			}

			if len(rows) == 0 {
				core.Print(nil, "(0 rows)")
				return core.Result{OK: true}
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
			line := core.NewBuilder()
			for i, col := range cols {
				if i > 0 {
					line.WriteString(" | ")
				}
				line.WriteString(core.Sprintf("%-*s", widths[i], truncate(col, widths[i])))
			}
			core.Print(nil, "%s", line.String())

			// Print separator.
			sep := core.NewBuilder()
			for i := range cols {
				if i > 0 {
					sep.WriteString("-+-")
				}
				sep.WriteString(repeatString("-", widths[i]))
			}
			core.Print(nil, "%s", sep.String())

			// Print rows.
			for _, row := range rows {
				b := core.NewBuilder()
				for i, col := range cols {
					if i > 0 {
						b.WriteString(" | ")
					}
					b.WriteString(core.Sprintf("%-*s", widths[i], truncate(formatValue(row[col]), widths[i])))
				}
				core.Print(nil, "%s", b.String())
			}

			core.Print(nil, "")
			core.Print(nil, "(%d rows)", len(rows))
			return core.Result{OK: true}
		},
	})
}

// formatValue renders a SQL row cell as a string — "NULL" for nil.
//
//	s := formatValue(row["id"])
func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
	return core.Sprintf("%v", v)
}

// truncate clips s to max chars, using "..." when truncation is needed.
//
//	t := truncate("hello world", 5) // "he..."
func truncate(s string, maximum int) string {
	if len(s) <= maximum {
		return s
	}
	if maximum <= 3 {
		return s[:maximum]
	}
	return s[:maximum-3] + "..."
}

// repeatString returns part concatenated count times.
//
//	bar := repeatString("-", 80)
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
