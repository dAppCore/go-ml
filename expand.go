package ml

import (
	"dappco.re/go/core"
	"context"
<<<<<<< HEAD
	"encoding/json"
=======
	"log"
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	"time"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
)

// ExpandOutput is the JSONL output structure for expansion generation.
type ExpandOutput struct {
	ID             string  `json:"id"`
	Domain         string  `json:"domain,omitempty"`
	Prompt         string  `json:"prompt"`
	Response       string  `json:"response"`
	Model          string  `json:"model"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	Chars          int     `json:"chars"`
}

// GetCompletedIDs queries InfluxDB for prompt IDs that have already been
// processed in the expansion_gen measurement.
func GetCompletedIDs(influx *InfluxClient) (map[string]bool, error) {
	rows, err := influx.QuerySQL("SELECT DISTINCT seed_id FROM expansion_gen")
	if err != nil {
		return nil, coreerr.E("ml.GetCompletedIDs", "query expansion_gen", err)
	}

	ids := make(map[string]bool, len(rows))
	for _, row := range rows {
		id := strVal(row, "seed_id")
		if id != "" {
			ids[id] = true
		}
	}

	return ids, nil
}

// ExpandPrompts generates responses for expansion prompts using the given
// backend and reports progress to InfluxDB. Already-completed prompts (per
// InfluxDB) are skipped. API errors for individual prompts are logged and
// skipped. InfluxDB reporting is best-effort.
func ExpandPrompts(ctx context.Context, backend Backend, influx *InfluxClient, prompts []Response,
	modelName, worker, outputDir string, dryRun bool, limit int) error {

	remaining := prompts

	// Check InfluxDB for already-completed IDs.
	completed, err := GetCompletedIDs(influx)
	if err != nil {
		core.Print(nil,"warning: could not check completed IDs: %v", err)
	} else {
		remaining = nil
		for _, p := range prompts {
			if !completed[p.ID] {
				remaining = append(remaining, p)
			}
		}

		skipped := len(prompts) - len(remaining)
		if skipped > 0 {
			core.Print(nil,"skipping %d already-completed prompts, %d remaining", skipped, len(remaining))
		}
	}

	if limit > 0 && limit < len(remaining) {
		remaining = remaining[:limit]
	}

	if len(remaining) == 0 {
		core.Print(nil,"all prompts already completed, nothing to do")
		return nil
	}

	if dryRun {
		core.Print(nil,"dry-run: would process %d prompts with model %s (worker: %s)", len(remaining), modelName, worker)
		for i, p := range remaining {
			if i >= 10 {
				core.Print(nil,"  ... and %d more", len(remaining)-10)
				break
			}
			core.Print(nil,"  %s (domain: %s)", p.ID, p.Domain)
		}
		return nil
	}

<<<<<<< HEAD
	outputPath := core.Path(outputDir, core.Sprintf("expand-%s.jsonl", worker))
=======
	outputPath := core.JoinPath(outputDir, core.Sprintf("expand-%s.jsonl", worker))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	f, err := coreio.Local.Append(outputPath)
	if err != nil {
		return coreerr.E("ml.ExpandPrompts", "open output file", err)
	}
	defer f.Close()

	total := len(remaining)
	completedCount := 0

	for idx, p := range remaining {
		start := time.Now()
		res, err := backend.Generate(ctx, p.Prompt, GenOpts{Temperature: 0.7, MaxTokens: 2048})
		elapsed := time.Since(start).Seconds()

		if err != nil {
			core.Print(nil,"[%d/%d] id=%s ERROR: %v", idx+1, total, p.ID, err)
			continue
		}

		response := res.Text
		chars := len(response)
		completedCount++

		out := ExpandOutput{
			ID:             p.ID,
			Domain:         p.Domain,
			Prompt:         p.Prompt,
			Response:       response,
			Model:          modelName,
			ElapsedSeconds: elapsed,
			Chars:          chars,
		}

<<<<<<< HEAD
		line, err := json.Marshal(out)
		if err != nil {
			core.Print(nil,"[%d/%d] id=%s marshal error: %v", idx+1, total, p.ID, err)
			continue
		}

		if _, err := f.Write(append(line, '\n')); err != nil {
			core.Print(nil,"[%d/%d] id=%s write error: %v", idx+1, total, p.ID, err)
=======
		line := core.JSONMarshalString(out)
		if _, err := f.Write([]byte(core.Concat(line, "\n"))); err != nil {
			log.Printf("[%d/%d] id=%s write error: %v", idx+1, total, p.ID, err)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
			continue
		}

		genLine := core.Sprintf("expansion_gen,i=%d,w=%s,d=%s seed_id=\"%s\",gen_time=%f,chars=%di,model=\"%s\"",
			idx, EscapeLp(worker), EscapeLp(p.Domain),
			p.ID, elapsed, chars, modelName)

		pct := float64(completedCount) / float64(total) * 100.0
		progressLine := core.Sprintf("expansion_progress,worker=%s completed=%di,target=%di,pct=%f",
			EscapeLp(worker), completedCount, total, pct)

		if writeErr := influx.WriteLp([]string{genLine, progressLine}); writeErr != nil {
			core.Print(nil,"[%d/%d] id=%s influx write error: %v", idx+1, total, p.ID, writeErr)
		}

		core.Print(nil,"[%d/%d] id=%s chars=%d time=%.1fs", idx+1, total, p.ID, chars, elapsed)
	}

	core.Print(nil,"expand complete: %d/%d prompts generated, output: %s", completedCount, total, outputPath)

	return nil
}
