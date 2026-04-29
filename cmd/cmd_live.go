package cmd

import (
	"dappco.re/go"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
)

// targetTotal is the target size of the golden set used for progress display.
const targetTotal = 15000

// addLiveCommand registers `ml live` — queries InfluxDB for real-time
// generation progress, worker breakdown, and domain/voice counts.
//
//	core ml live --influx http://10.69.69.165:8181 --influx-db training
func addLiveCommand(c *core.Core) {
	c.Command("ml/live", core.Command{
		Description: "Show live generation progress from InfluxDB",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			influx := ml.NewInfluxClient(influxURL, influxDB)

			// Total completed generations
			totalRows, err := influx.QuerySQL("SELECT count(DISTINCT i) AS n FROM gold_gen")
			if err != nil {
				return resultFromError(coreerr.E("cmd.runLive", "live: query total", err))
			}
			total := sqlScalar(totalRows)

			// Distinct domains and voices
			domainRows, err := influx.QuerySQL("SELECT count(DISTINCT d) AS n FROM gold_gen")
			if err != nil {
				return resultFromError(coreerr.E("cmd.runLive", "live: query domains", err))
			}
			domains := sqlScalar(domainRows)

			voiceRows, err := influx.QuerySQL("SELECT count(DISTINCT v) AS n FROM gold_gen")
			if err != nil {
				return resultFromError(coreerr.E("cmd.runLive", "live: query voices", err))
			}
			voices := sqlScalar(voiceRows)

			// Per-worker breakdown
			workers, err := influx.QuerySQL("SELECT w, count(DISTINCT i) AS n FROM gold_gen GROUP BY w ORDER BY n DESC")
			if err != nil {
				return resultFromError(coreerr.E("cmd.runLive", "live: query workers", err))
			}

			pct := float64(total) / float64(targetTotal) * 100
			remaining := targetTotal - total

			core.Print(nil, "Golden Set Live Status (from InfluxDB)")
			core.Print(nil, "─────────────────────────────────────────────")
			core.Print(nil, "  Total:     %d / %d (%.1f%%)", total, targetTotal, pct)
			core.Print(nil, "  Remaining: %d", remaining)
			core.Print(nil, "  Domains:   %d", domains)
			core.Print(nil, "  Voices:    %d", voices)
			core.Print(nil, "")
			core.Print(nil, "  Workers:")
			for _, w := range workers {
				name := w["w"]
				n := w["n"]
				marker := ""
				if name == "migration" {
					marker = " (seed data)"
				}
				core.Print(nil, "    %-20s %6s generations%s", name, n, marker)
			}

			return core.Result{OK: true}
		},
	})
}

// sqlScalar extracts the first numeric value from a QuerySQL result.
//
//	count := sqlScalar(rows)
func sqlScalar(rows []map[string]any) int {
	if len(rows) == 0 {
		return 0
	}
	for _, v := range rows[0] {
		return toInt(v)
	}
	return 0
}
