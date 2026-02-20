package ml

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// bufferEntry is a JSONL-buffered result for when InfluxDB is down.
type bufferEntry struct {
	Checkpoint Checkpoint  `json:"checkpoint"`
	Results    ProbeResult `json:"results"`
	Timestamp  string      `json:"timestamp"`
}

// ScoreCapabilityAndPush judges each capability response via LLM and pushes scores to InfluxDB.
func ScoreCapabilityAndPush(ctx context.Context, judge *Judge, influx *InfluxClient, cp Checkpoint, responses []CapResponseEntry) {
	const baseTS int64 = 1739577600
	var lines []string

	for i, cr := range responses {
		scores, err := judge.ScoreCapability(ctx, cr.Prompt, cr.Answer, cr.Response)
		if err != nil {
			log.Printf("  [%s] judge error: %v", cr.ProbeID, err)
			continue
		}

		avg := (scores.Reasoning + scores.Correctness + scores.Clarity) / 3.0
		log.Printf("  [%s] judge: R=%.1f C=%.1f Cl=%.1f avg=%.2f",
			cr.ProbeID, scores.Reasoning, scores.Correctness, scores.Clarity, avg)

		ts := (baseTS + int64(cp.Iteration)*1000 + int64(i)) * 1_000_000_000
		line := fmt.Sprintf(
			"capability_judge,model=%s,run_id=%s,label=%s,probe_id=%s,category=%s reasoning=%.2f,correctness=%.2f,clarity=%.2f,avg=%.2f,iteration=%di %d",
			EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label),
			EscapeLp(cr.ProbeID), EscapeLp(cr.Category),
			scores.Reasoning, scores.Correctness, scores.Clarity, avg, cp.Iteration, ts,
		)
		lines = append(lines, line)
	}

	if len(lines) > 0 {
		if err := influx.WriteLp(lines); err != nil {
			log.Printf("InfluxDB capability_judge push failed: %v", err)
		} else {
			log.Printf("Pushed %d capability judge scores to InfluxDB for %s", len(lines), cp.Label)
		}
	}
}

// ScoreContentAndPush scores content responses via judge and pushes scores to InfluxDB.
func ScoreContentAndPush(ctx context.Context, judge *Judge, influx *InfluxClient, cp Checkpoint, runID string, responses []ContentResponse) {
	const baseTS int64 = 1739577600
	dims := []string{"ccp_compliance", "truth_telling", "engagement", "axiom_integration", "sovereignty_reasoning", "emotional_register"}

	for i, cr := range responses {
		scores, err := judge.ScoreContent(ctx, cr.Probe, cr.Response)
		if err != nil {
			log.Printf("  [content:%s] judge error: %v", cr.Probe.ID, err)
			continue
		}

		log.Printf("  [content:%s] ccp=%d truth=%d engage=%d axiom=%d sov=%d emot=%d",
			cr.Probe.ID,
			scores.CCPCompliance, scores.TruthTelling, scores.Engagement,
			scores.AxiomIntegration, scores.SovereigntyReasoning, scores.EmotionalRegister)

		scoreMap := map[string]int{
			"ccp_compliance":        scores.CCPCompliance,
			"truth_telling":         scores.TruthTelling,
			"engagement":            scores.Engagement,
			"axiom_integration":     scores.AxiomIntegration,
			"sovereignty_reasoning": scores.SovereigntyReasoning,
			"emotional_register":    scores.EmotionalRegister,
		}

		var lines []string
		for j, dim := range dims {
			val := scoreMap[dim]
			ts := (baseTS + int64(cp.Iteration)*1000 + int64(i*10+j)) * 1_000_000_000
			line := fmt.Sprintf(
				"content_score,model=%s,run_id=%s,label=%s,dimension=%s,has_kernel=true score=%d,iteration=%di %d",
				EscapeLp(cp.ModelTag), EscapeLp(runID), EscapeLp(cp.Label), EscapeLp(dim),
				val, cp.Iteration, ts,
			)
			lines = append(lines, line)
		}

		if err := influx.WriteLp(lines); err != nil {
			log.Printf("  [content:%s] InfluxDB push failed: %v", cr.Probe.ID, err)
		}
	}

	log.Printf("Content scoring done for %s: %d probes × %d dimensions", cp.Label, len(responses), len(dims))
}

// PushCapabilitySummary pushes overall + per-category scores to InfluxDB.
func PushCapabilitySummary(influx *InfluxClient, cp Checkpoint, results ProbeResult) error {
	const baseTS int64 = 1739577600

	var lines []string

	ts := (baseTS + int64(cp.Iteration)*1000 + 0) * 1_000_000_000
	lines = append(lines, fmt.Sprintf(
		"capability_score,model=%s,run_id=%s,label=%s,category=overall accuracy=%.1f,correct=%di,total=%di,iteration=%di %d",
		EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label),
		results.Accuracy, results.Correct, results.Total, cp.Iteration, ts,
	))

	cats := make([]string, 0, len(results.ByCategory))
	for cat := range results.ByCategory {
		cats = append(cats, cat)
	}
	sort.Strings(cats)

	for i, cat := range cats {
		data := results.ByCategory[cat]
		catAcc := 0.0
		if data.Total > 0 {
			catAcc = float64(data.Correct) / float64(data.Total) * 100
		}
		ts := (baseTS + int64(cp.Iteration)*1000 + int64(i+1)) * 1_000_000_000
		lines = append(lines, fmt.Sprintf(
			"capability_score,model=%s,run_id=%s,label=%s,category=%s accuracy=%.1f,correct=%di,total=%di,iteration=%di %d",
			EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label), EscapeLp(cat),
			catAcc, data.Correct, data.Total, cp.Iteration, ts,
		))
	}

	if err := influx.WriteLp(lines); err != nil {
		return err
	}
	log.Printf("Pushed %d summary points to InfluxDB for %s", len(lines), cp.Label)
	return nil
}

// PushCapabilityResults pushes all results (overall + categories + probes) in one batch.
func PushCapabilityResults(influx *InfluxClient, cp Checkpoint, results ProbeResult) error {
	const baseTS int64 = 1739577600

	var lines []string

	ts := (baseTS + int64(cp.Iteration)*1000 + 0) * 1_000_000_000
	lines = append(lines, fmt.Sprintf(
		"capability_score,model=%s,run_id=%s,label=%s,category=overall accuracy=%.1f,correct=%di,total=%di,iteration=%di %d",
		EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label),
		results.Accuracy, results.Correct, results.Total, cp.Iteration, ts,
	))

	cats := make([]string, 0, len(results.ByCategory))
	for cat := range results.ByCategory {
		cats = append(cats, cat)
	}
	sort.Strings(cats)

	for i, cat := range cats {
		data := results.ByCategory[cat]
		catAcc := 0.0
		if data.Total > 0 {
			catAcc = float64(data.Correct) / float64(data.Total) * 100
		}
		ts := (baseTS + int64(cp.Iteration)*1000 + int64(i+1)) * 1_000_000_000
		lines = append(lines, fmt.Sprintf(
			"capability_score,model=%s,run_id=%s,label=%s,category=%s accuracy=%.1f,correct=%di,total=%di,iteration=%di %d",
			EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label), EscapeLp(cat),
			catAcc, data.Correct, data.Total, cp.Iteration, ts,
		))
	}

	probeIDs := make([]string, 0, len(results.Probes))
	for id := range results.Probes {
		probeIDs = append(probeIDs, id)
	}
	sort.Strings(probeIDs)

	for j, probeID := range probeIDs {
		probeRes := results.Probes[probeID]
		passedInt := 0
		if probeRes.Passed {
			passedInt = 1
		}
		ts := (baseTS + int64(cp.Iteration)*1000 + int64(j+100)) * 1_000_000_000
		lines = append(lines, fmt.Sprintf(
			"probe_score,model=%s,run_id=%s,label=%s,probe_id=%s passed=%di,iteration=%di %d",
			EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label), EscapeLp(probeID),
			passedInt, cp.Iteration, ts,
		))
	}

	if err := influx.WriteLp(lines); err != nil {
		return err
	}
	log.Printf("Pushed %d points to InfluxDB for %s", len(lines), cp.Label)
	return nil
}

// PushCapabilityResultsDB writes scoring results to DuckDB for persistent storage.
func PushCapabilityResultsDB(dbPath string, cp Checkpoint, results ProbeResult) {
	if dbPath == "" {
		return
	}

	db, err := OpenDBReadWrite(dbPath)
	if err != nil {
		log.Printf("DuckDB dual-write: open failed: %v", err)
		return
	}
	defer db.Close()

	db.EnsureScoringTables()

	_, err = db.conn.Exec(
		`INSERT OR REPLACE INTO checkpoint_scores (model, run_id, label, iteration, correct, total, accuracy)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		cp.ModelTag, cp.RunID, cp.Label, cp.Iteration,
		results.Correct, results.Total, results.Accuracy,
	)
	if err != nil {
		log.Printf("DuckDB dual-write: checkpoint_scores insert: %v", err)
	}

	for probeID, probeRes := range results.Probes {
		db.conn.Exec(
			`INSERT OR REPLACE INTO probe_results (model, run_id, label, probe_id, passed, response, iteration)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			cp.ModelTag, cp.RunID, cp.Label, probeID,
			probeRes.Passed, probeRes.Response, cp.Iteration,
		)
	}

	log.Printf("DuckDB: wrote %d probe results for %s", len(results.Probes)+1, cp.Label)
}

// BufferInfluxResult saves results to a local JSONL file when InfluxDB is down.
func BufferInfluxResult(workDir string, cp Checkpoint, results ProbeResult) {
	bufPath := filepath.Join(workDir, "influx_buffer.jsonl")
	f, err := os.OpenFile(bufPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Cannot open buffer file: %v", err)
		return
	}
	defer f.Close()

	entry := bufferEntry{
		Checkpoint: cp,
		Results:    results,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(entry)
	f.Write(append(data, '\n'))
	log.Printf("Buffered results to %s", bufPath)
}

// ReplayInfluxBuffer retries pushing buffered results to InfluxDB.
func ReplayInfluxBuffer(workDir string, influx *InfluxClient) {
	bufPath := filepath.Join(workDir, "influx_buffer.jsonl")
	data, err := os.ReadFile(bufPath)
	if err != nil {
		return
	}

	var remaining []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry bufferEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			remaining = append(remaining, line)
			continue
		}
		if err := PushCapabilityResults(influx, entry.Checkpoint, entry.Results); err != nil {
			remaining = append(remaining, line)
		} else {
			log.Printf("Replayed buffered result: %s", entry.Checkpoint.Label)
		}
	}

	if len(remaining) > 0 {
		os.WriteFile(bufPath, []byte(strings.Join(remaining, "\n")+"\n"), 0644)
	} else {
		os.Remove(bufPath)
		log.Println("Buffer fully replayed and cleared")
	}
}
