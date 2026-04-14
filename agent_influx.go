package ml

import (
	"context"
	"maps"
	"slices"
	"time"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
	"dappco.re/go/core/store"
)

// bufferEntry is a JSONL-buffered result for when InfluxDB is down.
type bufferEntry struct {
	Checkpoint Checkpoint  `json:"checkpoint"`
	Results    ProbeResult `json:"results"`
	Timestamp  string      `json:"timestamp"`
}

// ScoreCapabilityAndPush judges each capability response via LLM and pushes scores to InfluxDB.
func ScoreCapabilityAndPush(ctx context.Context, judge *Judge, influx *InfluxClient, cp Checkpoint, responses []CapResponseEntry) {
	var lines []string

	for i, cr := range responses {
		scores, err := judge.ScoreCapability(ctx, cr.Prompt, cr.Answer, cr.Response)
		if err != nil {
			core.Print(nil,"  [%s] judge error: %v", cr.ProbeID, err)
			continue
		}

		avg := (scores.Reasoning + scores.Correctness + scores.Clarity) / 3.0
		core.Print(nil,"  [%s] judge: R=%.1f C=%.1f Cl=%.1f avg=%.2f",
			cr.ProbeID, scores.Reasoning, scores.Correctness, scores.Clarity, avg)

		ts := (EpochBase + int64(cp.Iteration)*1000 + int64(i)) * 1_000_000_000
		line := core.Sprintf(
			MeasurementCapabilityJudge+",model=%s,run_id=%s,label=%s,probe_id=%s,category=%s reasoning=%.2f,correctness=%.2f,clarity=%.2f,avg=%.2f,iteration=%di %d",
			EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label),
			EscapeLp(cr.ProbeID), EscapeLp(cr.Category),
			scores.Reasoning, scores.Correctness, scores.Clarity, avg, cp.Iteration, ts,
		)
		lines = append(lines, line)
	}

	if len(lines) > 0 {
		if err := influx.WriteLp(lines); err != nil {
			core.Print(nil,"InfluxDB %s push failed: %v", MeasurementCapabilityJudge, err)
		} else {
			core.Print(nil,"Pushed %d capability judge scores to InfluxDB for %s", len(lines), cp.Label)
		}
	}
}

// ScoreContentAndPush scores content responses via judge and pushes scores to InfluxDB.
func ScoreContentAndPush(ctx context.Context, judge *Judge, influx *InfluxClient, cp Checkpoint, runID string, responses []ContentResponse) {
	dims := []string{"ccp_compliance", "truth_telling", "engagement", "axiom_integration", "sovereignty_reasoning", "emotional_register"}

	for i, cr := range responses {
		scores, err := judge.ScoreContent(ctx, cr.Probe, cr.Response)
		if err != nil {
			core.Print(nil,"  [content:%s] judge error: %v", cr.Probe.ID, err)
			continue
		}

		core.Print(nil,"  [content:%s] ccp=%d truth=%d engage=%d axiom=%d sov=%d emot=%d",
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
			ts := (EpochBase + int64(cp.Iteration)*1000 + int64(i*10+j)) * 1_000_000_000
			line := core.Sprintf(
				MeasurementContentScore+",model=%s,run_id=%s,label=%s,dimension=%s,has_kernel=true score=%d,iteration=%di %d",
				EscapeLp(cp.ModelTag), EscapeLp(runID), EscapeLp(cp.Label), EscapeLp(dim),
				val, cp.Iteration, ts,
			)
			lines = append(lines, line)
		}

		if err := influx.WriteLp(lines); err != nil {
			core.Print(nil,"  [content:%s] InfluxDB push failed: %v", cr.Probe.ID, err)
		}
	}

	core.Print(nil,"Content scoring done for %s: %d probes x %d dimensions", cp.Label, len(responses), len(dims))
}

// PushCapabilitySummary pushes overall + per-category scores to InfluxDB.
func PushCapabilitySummary(influx *InfluxClient, cp Checkpoint, results ProbeResult) error {
	var lines []string

	ts := (EpochBase + int64(cp.Iteration)*1000 + 0) * 1_000_000_000
	lines = append(lines, core.Sprintf(
		MeasurementCapabilityScore+",model=%s,run_id=%s,label=%s,category=overall accuracy=%.1f,correct=%di,total=%di,iteration=%di %d",
		EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label),
		results.Accuracy, results.Correct, results.Total, cp.Iteration, ts,
	))

	cats := slices.Sorted(maps.Keys(results.ByCategory))

	for i, cat := range cats {
		data := results.ByCategory[cat]
		catAcc := 0.0
		if data.Total > 0 {
			catAcc = float64(data.Correct) / float64(data.Total) * 100
		}
		ts := (EpochBase + int64(cp.Iteration)*1000 + int64(i+1)) * 1_000_000_000
		lines = append(lines, core.Sprintf(
			MeasurementCapabilityScore+",model=%s,run_id=%s,label=%s,category=%s accuracy=%.1f,correct=%di,total=%di,iteration=%di %d",
			EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label), EscapeLp(cat),
			catAcc, data.Correct, data.Total, cp.Iteration, ts,
		))
	}

	if err := influx.WriteLp(lines); err != nil {
		return err
	}
	core.Print(nil,"Pushed %d summary points to InfluxDB for %s", len(lines), cp.Label)
	return nil
}

// PushCapabilityResults pushes all results (overall + categories + probes) in one batch.
func PushCapabilityResults(influx *InfluxClient, cp Checkpoint, results ProbeResult) error {
	var lines []string

	ts := (EpochBase + int64(cp.Iteration)*1000 + 0) * 1_000_000_000
	lines = append(lines, core.Sprintf(
		MeasurementCapabilityScore+",model=%s,run_id=%s,label=%s,category=overall accuracy=%.1f,correct=%di,total=%di,iteration=%di %d",
		EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label),
		results.Accuracy, results.Correct, results.Total, cp.Iteration, ts,
	))

	cats := slices.Sorted(maps.Keys(results.ByCategory))

	for i, cat := range cats {
		data := results.ByCategory[cat]
		catAcc := 0.0
		if data.Total > 0 {
			catAcc = float64(data.Correct) / float64(data.Total) * 100
		}
		ts := (EpochBase + int64(cp.Iteration)*1000 + int64(i+1)) * 1_000_000_000
		lines = append(lines, core.Sprintf(
			MeasurementCapabilityScore+",model=%s,run_id=%s,label=%s,category=%s accuracy=%.1f,correct=%di,total=%di,iteration=%di %d",
			EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label), EscapeLp(cat),
			catAcc, data.Correct, data.Total, cp.Iteration, ts,
		))
	}

	probeIDs := slices.Sorted(maps.Keys(results.Probes))

	for j, probeID := range probeIDs {
		probeRes := results.Probes[probeID]
		passedInt := 0
		if probeRes.Passed {
			passedInt = 1
		}
		ts := (EpochBase + int64(cp.Iteration)*1000 + int64(j+100)) * 1_000_000_000
		lines = append(lines, core.Sprintf(
			MeasurementProbeScore+",model=%s,run_id=%s,label=%s,probe_id=%s passed=%di,iteration=%di %d",
			EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label), EscapeLp(probeID),
			passedInt, cp.Iteration, ts,
		))
	}

	if err := influx.WriteLp(lines); err != nil {
		return err
	}
	core.Print(nil,"Pushed %d points to InfluxDB for %s", len(lines), cp.Label)
	return nil
}

// PushCapabilityResultsDB writes scoring results to DuckDB for persistent storage.
func PushCapabilityResultsDB(dbPath string, cp Checkpoint, results ProbeResult) {
	if dbPath == "" {
		return
	}

	db, err := store.OpenDuckDBReadWrite(dbPath)
	if err != nil {
		core.Print(nil,"DuckDB dual-write: open failed: %v", err)
		return
	}
	defer db.Close()

	db.EnsureScoringTables()

	err = db.Exec(
		core.Sprintf(`INSERT OR REPLACE INTO %s (model, run_id, label, iteration, correct, total, accuracy)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`, TableCheckpointScores),
		cp.ModelTag, cp.RunID, cp.Label, cp.Iteration,
		results.Correct, results.Total, results.Accuracy,
	)
	if err != nil {
		core.Print(nil,"DuckDB dual-write: %s insert: %v", TableCheckpointScores, err)
	}

	for probeID, probeRes := range results.Probes {
		db.Exec(
			core.Sprintf(`INSERT OR REPLACE INTO %s (model, run_id, label, probe_id, passed, response, iteration)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`, TableProbeResults),
			cp.ModelTag, cp.RunID, cp.Label, probeID,
			probeRes.Passed, probeRes.Response, cp.Iteration,
		)
	}

	core.Print(nil,"DuckDB: wrote %d probe results for %s", len(results.Probes)+1, cp.Label)
}

// BufferInfluxResult saves results to a local JSONL file when InfluxDB is down.
func BufferInfluxResult(workDir string, cp Checkpoint, results ProbeResult) {
	bufPath := core.JoinPath(workDir, InfluxBufferFile)
	f, err := coreio.Local.Append(bufPath)
	if err != nil {
		core.Print(nil,"Cannot open buffer file: %v", err)
		return
	}
	defer f.Close()

	entry := bufferEntry{
		Checkpoint: cp,
		Results:    results,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	f.Write([]byte(core.Concat(core.JSONMarshalString(entry), "\n")))
	core.Print(nil, "Buffered results to %s", bufPath)
}

// ReplayInfluxBuffer retries pushing buffered results to InfluxDB.
func ReplayInfluxBuffer(workDir string, influx *InfluxClient) {
	bufPath := core.JoinPath(workDir, InfluxBufferFile)
	data, err := coreio.Local.Read(bufPath)
	if err != nil {
		return
	}

	var remaining []string
	for _, line := range core.Split(core.Trim(data), "\n") {
		if line == "" {
			continue
		}
		var entry bufferEntry
		if r := core.JSONUnmarshalString(line, &entry); !r.OK {
			remaining = append(remaining, line)
			continue
		}
		if err := PushCapabilityResults(influx, entry.Checkpoint, entry.Results); err != nil {
			remaining = append(remaining, line)
		} else {
			core.Print(nil,"Replayed buffered result: %s", entry.Checkpoint.Label)
		}
	}

	if len(remaining) > 0 {
		coreio.Local.Write(bufPath, core.Concat(core.Join("\n", remaining...), "\n"))
	} else {
		coreio.Local.Delete(bufPath)
		core.Print(nil,"Buffer fully replayed and cleared")
	}
}
