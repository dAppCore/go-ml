package cmd

import (
	"fmt"
	"os"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"forge.lthn.ai/core/cli/pkg/cli"
)

const targetTotal = 15000

var liveCmd = &cli.Command{
	Use:   "live",
	Short: "Show live generation progress from InfluxDB",
	Long:  "Queries InfluxDB for real-time generation progress, worker breakdown, and domain/voice counts.",
	RunE:  runLive,
}

func runLive(cmd *cli.Command, args []string) error {
	influx := ml.NewInfluxClient(influxURL, influxDB)

	// Total completed generations
	totalRows, err := influx.QuerySQL("SELECT count(DISTINCT i) AS n FROM gold_gen")
	if err != nil {
		return coreerr.E("cmd.runLive", "live: query total", err)
	}
	total := sqlScalar(totalRows)

	// Distinct domains and voices
	domainRows, err := influx.QuerySQL("SELECT count(DISTINCT d) AS n FROM gold_gen")
	if err != nil {
		return coreerr.E("cmd.runLive", "live: query domains", err)
	}
	domains := sqlScalar(domainRows)

	voiceRows, err := influx.QuerySQL("SELECT count(DISTINCT v) AS n FROM gold_gen")
	if err != nil {
		return coreerr.E("cmd.runLive", "live: query voices", err)
	}
	voices := sqlScalar(voiceRows)

	// Per-worker breakdown
	workers, err := influx.QuerySQL("SELECT w, count(DISTINCT i) AS n FROM gold_gen GROUP BY w ORDER BY n DESC")
	if err != nil {
		return coreerr.E("cmd.runLive", "live: query workers", err)
	}

	pct := float64(total) / float64(targetTotal) * 100
	remaining := targetTotal - total

	fmt.Fprintln(os.Stdout, "Golden Set Live Status (from InfluxDB)")
	fmt.Fprintln(os.Stdout, "─────────────────────────────────────────────")
	fmt.Fprintf(os.Stdout, "  Total:     %d / %d (%.1f%%)\n", total, targetTotal, pct)
	fmt.Fprintf(os.Stdout, "  Remaining: %d\n", remaining)
	fmt.Fprintf(os.Stdout, "  Domains:   %d\n", domains)
	fmt.Fprintf(os.Stdout, "  Voices:    %d\n", voices)
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "  Workers:")
	for _, w := range workers {
		name := w["w"]
		n := w["n"]
		marker := ""
		if name == "migration" {
			marker = " (seed data)"
		}
		fmt.Fprintf(os.Stdout, "    %-20s %6s generations%s\n", name, n, marker)
	}

	return nil
}

// sqlScalar extracts the first numeric value from a QuerySQL result.
func sqlScalar(rows []map[string]any) int {
	if len(rows) == 0 {
		return 0
	}
	for _, v := range rows[0] {
		return toInt(v)
	}
	return 0
}
