package cmd

import (
	"dappco.re/go"
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
			totalResult := influx.QuerySQL("SELECT count(DISTINCT i) AS n FROM gold_gen")
			if !totalResult.OK {
				return core.Fail(core.E("cmd.runLive", "live: query total", totalResult.Value.(error)))
			}
			totalRows := totalResult.Value.([]map[string]any)
			total := sqlScalar(totalRows)

			// Distinct domains and voices
			domainResult := influx.QuerySQL("SELECT count(DISTINCT d) AS n FROM gold_gen")
			if !domainResult.OK {
				return core.Fail(core.E("cmd.runLive", "live: query domains", domainResult.Value.(error)))
			}
			domainRows := domainResult.Value.([]map[string]any)
			domains := sqlScalar(domainRows)

			voiceResult := influx.QuerySQL("SELECT count(DISTINCT v) AS n FROM gold_gen")
			if !voiceResult.OK {
				return core.Fail(core.E("cmd.runLive", "live: query voices", voiceResult.Value.(error)))
			}
			voiceRows := voiceResult.Value.([]map[string]any)
			voices := sqlScalar(voiceRows)

			// Per-worker breakdown
			workerResult := influx.QuerySQL("SELECT w, count(DISTINCT i) AS n FROM gold_gen GROUP BY w ORDER BY n DESC")
			if !workerResult.OK {
				return core.Fail(core.E("cmd.runLive", "live: query workers", workerResult.Value.(error)))
			}
			workers := workerResult.Value.([]map[string]any)

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

			return core.Ok(nil)
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
