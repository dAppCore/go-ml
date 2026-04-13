package ml

import (
	"bufio"
	"context"
<<<<<<< HEAD
	"encoding/json"
	"io"
=======
	"io"
	"log"
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
)

// ProbeResult holds the result of running all probes against a checkpoint.
type ProbeResult struct {
	Accuracy   float64                      `json:"accuracy"`
	Correct    int                          `json:"correct"`
	Total      int                          `json:"total"`
	ByCategory map[string]CategoryResult    `json:"by_category"`
	Probes     map[string]SingleProbeResult `json:"probes"`
}

// CategoryResult holds pass/fail counts for a probe category.
type CategoryResult struct {
	Correct int `json:"correct"`
	Total   int `json:"total"`
}

// SingleProbeResult holds the result of a single probe.
type SingleProbeResult struct {
	Passed   bool   `json:"passed"`
	Response string `json:"response"`
}

// ProbeCallback is called after each probe completes for real-time streaming.
type ProbeCallback func(probeID, category string, passed bool, response string, correct, total int)

// CapResponseEntry holds a capability probe response with its metadata for judge scoring.
type CapResponseEntry struct {
	ProbeID  string
	Category string
	Prompt   string
	Answer   string
	Response string
	Passed   bool
}

// ContentResponse holds a content probe response for later judging.
type ContentResponse struct {
	Probe    ContentProbe
	Response string
}

// probeRunnerResponse is the JSON response from the Python probe runner.
type probeRunnerResponse struct {
	Response string  `json:"response"`
	Error    string  `json:"error"`
	Elapsed  float64 `json:"elapsed"`
}

// processMLXNative scores a checkpoint using Ollama on M3.
func processMLXNative(cfg *AgentConfig, influx *InfluxClient, cp Checkpoint) error {
	ollamaBase, ok := OllamaBaseModelMap[cp.ModelTag]
	if !ok {
		return coreerr.E("ml.processMLXNative", core.Sprintf("unknown Ollama model for tag %s", cp.ModelTag), nil)
	}
	hfBase := HFBaseModelMap[cp.ModelTag]
	if hfBase == "" {
		hfBase = ollamaBase
	}

	tempModel := core.Sprintf("lem-%s-%d", cp.ModelTag, cp.Iteration)
<<<<<<< HEAD
	localAdapterDir := core.Path(cfg.WorkDir, "adapter-"+cp.Dirname)
	peftDir := core.Path(cfg.WorkDir, "peft-"+cp.Dirname)
=======
	localAdapterDir := core.JoinPath(cfg.WorkDir, core.Concat("adapter-", cp.Dirname))
	peftDir := core.JoinPath(cfg.WorkDir, core.Concat("peft-", cp.Dirname))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

	coreio.Local.EnsureDir(localAdapterDir)

	defer func() {
		coreio.Local.DeleteAll(localAdapterDir)
		coreio.Local.DeleteAll(peftDir)
		OllamaDeleteModel(cfg.JudgeURL, tempModel)
	}()

<<<<<<< HEAD
	core.Print(nil,"Fetching adapter from M3 (%s)...", cp.Filename)
	remoteSF := core.Sprintf("%s/%s", cp.RemoteDir, cp.Filename)
	remoteCfg := core.Sprintf("%s/adapter_config.json", cp.RemoteDir)
	localSF := core.Path(localAdapterDir, cp.Filename)
	localCfg := core.Path(localAdapterDir, "adapter_config.json")
=======
	log.Printf("Fetching adapter from M3 (%s)...", cp.Filename)
	remoteSF := core.Sprintf("%s/%s", cp.RemoteDir, cp.Filename)
	remoteCfg := core.Sprintf("%s/adapter_config.json", cp.RemoteDir)
	localSF := core.JoinPath(localAdapterDir, cp.Filename)
	localCfg := core.JoinPath(localAdapterDir, "adapter_config.json")
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

	ctx := context.Background()
	t := cfg.transport()
	if err := t.CopyFrom(ctx, remoteSF, localSF); err != nil {
		return coreerr.E("ml.processMLXNative", "scp safetensors", err)
	}
	if err := t.CopyFrom(ctx, remoteCfg, localCfg); err != nil {
		return coreerr.E("ml.processMLXNative", "scp config", err)
	}

	core.Print(nil,"Converting MLX → PEFT format...")
	if err := ConvertMLXtoPEFT(localSF, localCfg, peftDir, hfBase); err != nil {
		return coreerr.E("ml.processMLXNative", "convert adapter", err)
	}

	core.Print(nil,"Creating Ollama model %s (base: %s)...", tempModel, ollamaBase)
	if err := OllamaCreateModel(cfg.JudgeURL, tempModel, ollamaBase, peftDir); err != nil {
		return coreerr.E("ml.processMLXNative", "ollama create", err)
	}
	core.Print(nil,"Ollama model %s ready", tempModel)
	probeBackend := NewHTTPBackend(cfg.JudgeURL, tempModel)

	results, fullResponses := RunCapabilityProbesFull(ctx, probeBackend, func(probeID, category string, passed bool, response string, correct, total int) {
		passedInt := 0
		if passed {
			passedInt = 1
		}
		ts := (EpochBase + int64(cp.Iteration)*1000 + int64(total+100)) * 1_000_000_000
		line := core.Sprintf(
			MeasurementProbeScore+",model=%s,run_id=%s,label=%s,probe_id=%s passed=%di,iteration=%di %d",
			EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label), EscapeLp(probeID),
			passedInt, cp.Iteration, ts,
		)
		if err := influx.WriteLp([]string{line}); err != nil {
			core.Print(nil,"  [%s] InfluxDB stream failed: %v", probeID, err)
		}
	})

	core.Print(nil,"Capability: %s -- %.1f%% (%d/%d)",
		cp.Label, results.Accuracy, results.Correct, results.Total)

	if err := PushCapabilitySummary(influx, cp, results); err != nil {
		core.Print(nil,"InfluxDB summary push failed, buffering: %v", err)
		BufferInfluxResult(cfg.WorkDir, cp, results)
	}
	PushCapabilityResultsDB(cfg.DBPath, cp, results)

	judgeBackend := NewHTTPBackend(cfg.JudgeURL, cfg.JudgeModel)
	judge := NewJudge(judgeBackend)

	core.Print(nil,"Judging %d capability responses (0-10 quality scoring)...", len(fullResponses))
	ScoreCapabilityAndPush(ctx, judge, influx, cp, fullResponses)

	core.Print(nil,"Running %d content probes (0-10 judge scoring)...", len(ContentProbes))
	contentResponses := RunContentProbesViaAPI(ctx, probeBackend)
	if len(contentResponses) > 0 {
		contentRunID := core.Replace(cp.RunID, "-capability-", "-content-")
		ScoreContentAndPush(ctx, judge, influx, cp, contentRunID, contentResponses)
	}

	return nil
}

// processWithConversion fetches adapter locally, converts MLX→PEFT, and scores.
func processWithConversion(cfg *AgentConfig, influx *InfluxClient, cp Checkpoint) error {
<<<<<<< HEAD
	localAdapterDir := core.Path(cfg.WorkDir, cp.Dirname)
	coreio.Local.EnsureDir(localAdapterDir)

	localSF := core.Path(localAdapterDir, cp.Filename)
	localCfg := core.Path(localAdapterDir, "adapter_config.json")
=======
	localAdapterDir := core.JoinPath(cfg.WorkDir, cp.Dirname)
	coreio.Local.EnsureDir(localAdapterDir)

	localSF := core.JoinPath(localAdapterDir, cp.Filename)
	localCfg := core.JoinPath(localAdapterDir, "adapter_config.json")
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

	defer func() {
		coreio.Local.Delete(localSF)
		coreio.Local.Delete(localCfg)
<<<<<<< HEAD
		peftDir := core.Path(cfg.WorkDir, core.Sprintf("peft_%07d", cp.Iteration))
		coreio.Local.DeleteAll(peftDir)
	}()

	core.Print(nil,"Fetching adapter from M3...")
=======
		peftDir := core.JoinPath(cfg.WorkDir, core.Sprintf("peft_%07d", cp.Iteration))
		coreio.Local.DeleteAll(peftDir)
	}()

	log.Println("Fetching adapter from M3...")
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	remoteSF := core.Sprintf("%s/%s", cp.RemoteDir, cp.Filename)
	remoteCfg := core.Sprintf("%s/adapter_config.json", cp.RemoteDir)

	ctx := context.Background()
	t := cfg.transport()
	if err := t.CopyFrom(ctx, remoteSF, localSF); err != nil {
		return coreerr.E("ml.processWithConversion", "scp safetensors", err)
	}
	if err := t.CopyFrom(ctx, remoteCfg, localCfg); err != nil {
		return coreerr.E("ml.processWithConversion", "scp config", err)
	}

<<<<<<< HEAD
	core.Print(nil,"Converting MLX to PEFT format...")
	peftDir := core.Path(cfg.WorkDir, core.Sprintf("peft_%07d", cp.Iteration))
=======
	log.Println("Converting MLX to PEFT format...")
	peftDir := core.JoinPath(cfg.WorkDir, core.Sprintf("peft_%07d", cp.Iteration))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	if err := ConvertMLXtoPEFT(localSF, localCfg, peftDir, cfg.BaseModel); err != nil {
		return coreerr.E("ml.processWithConversion", "convert adapter", err)
	}

	core.Print(nil,"Running %d capability probes...", len(CapabilityProbes))
	modelName := cfg.Model
	if modelName == "" {
		modelName = cp.ModelTag
	}
	backend := NewHTTPBackend(cfg.APIURL, modelName)

	results := RunCapabilityProbes(ctx, backend)

	core.Print(nil,"Result: %s -- %.1f%% (%d/%d)",
		cp.Label, results.Accuracy, results.Correct, results.Total)

	if err := PushCapabilityResults(influx, cp, results); err != nil {
		core.Print(nil,"InfluxDB push failed, buffering: %v", err)
		BufferInfluxResult(cfg.WorkDir, cp, results)
	}
	PushCapabilityResultsDB(cfg.DBPath, cp, results)

	return nil
}

// RunCapabilityProbes runs all capability probes against a backend.
func RunCapabilityProbes(ctx context.Context, backend Backend) ProbeResult {
	results := ProbeResult{
		ByCategory: make(map[string]CategoryResult),
		Probes:     make(map[string]SingleProbeResult),
	}

	correct := 0
	total := 0

	for _, probe := range CapabilityProbes {
		res, err := backend.Generate(ctx, probe.Prompt, GenOpts{Temperature: CapabilityTemperature, MaxTokens: CapabilityMaxTokens})
		if err != nil {
			core.Print(nil,"  [%s] ERROR: %v", probe.ID, err)
			results.Probes[probe.ID] = SingleProbeResult{Passed: false, Response: err.Error()}
			total++
			cat := results.ByCategory[probe.Category]
			cat.Total++
			results.ByCategory[probe.Category] = cat
			continue
		}

		clean := StripThinkBlocks(res.Text)
		passed := probe.Check(clean)
		total++
		if passed {
			correct++
		}

		cat := results.ByCategory[probe.Category]
		cat.Total++
		if passed {
			cat.Correct++
		}
		results.ByCategory[probe.Category] = cat

		stored := clean
		if len(stored) > MaxStoredResponseLen {
			stored = stored[:MaxStoredResponseLen]
		}
		results.Probes[probe.ID] = SingleProbeResult{Passed: passed, Response: stored}

		status := "FAIL"
		if passed {
			status = "PASS"
		}
		core.Print(nil,"  [%s] %s (expected: %s)", probe.ID, status, probe.Answer)
	}

	if total > 0 {
		results.Accuracy = float64(correct) / float64(total) * 100
	}
	results.Correct = correct
	results.Total = total

	return results
}

// RunCapabilityProbesFull runs all probes via a backend and returns both
// aggregate results and full responses for judge scoring.
func RunCapabilityProbesFull(ctx context.Context, backend Backend, onProbe ProbeCallback) (ProbeResult, []CapResponseEntry) {
	results := ProbeResult{
		ByCategory: make(map[string]CategoryResult),
		Probes:     make(map[string]SingleProbeResult),
	}
	var fullResponses []CapResponseEntry

	correct := 0
	total := 0

	for _, probe := range CapabilityProbes {
		res, err := backend.Generate(ctx, probe.Prompt, GenOpts{Temperature: CapabilityTemperature, MaxTokens: CapabilityMaxTokens})
		response := res.Text
		if err != nil {
<<<<<<< HEAD
			core.Print(nil,"  [%s] ERROR: %v", probe.ID, err)
=======
			log.Printf("  [%s] ERROR: %v", probe.ID, err)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
			response = core.Sprintf("ERROR: %v", err)
		}

		clean := StripThinkBlocks(response)
		passed := probe.Check(clean)
		total++
		if passed {
			correct++
		}

		cat := results.ByCategory[probe.Category]
		cat.Total++
		if passed {
			cat.Correct++
		}
		results.ByCategory[probe.Category] = cat

		stored := clean
		if len(stored) > MaxStoredResponseLen {
			stored = stored[:MaxStoredResponseLen]
		}
		results.Probes[probe.ID] = SingleProbeResult{Passed: passed, Response: stored}

		fullResponses = append(fullResponses, CapResponseEntry{
			ProbeID:  probe.ID,
			Category: probe.Category,
			Prompt:   probe.Prompt,
			Answer:   probe.Answer,
			Response: clean,
			Passed:   passed,
		})

		status := "FAIL"
		if passed {
			status = "PASS"
		}
		core.Print(nil,"  [%s] %s (expected: %s)", probe.ID, status, probe.Answer)

		if onProbe != nil {
			onProbe(probe.ID, probe.Category, passed, stored, correct, total)
		}
	}

	if total > 0 {
		results.Accuracy = float64(correct) / float64(total) * 100
	}
	results.Correct = correct
	results.Total = total

	return results, fullResponses
}

// RunContentProbesViaAPI runs content probes via a backend.
func RunContentProbesViaAPI(ctx context.Context, backend Backend) []ContentResponse {
	var responses []ContentResponse

	for _, probe := range ContentProbes {
		res, err := backend.Generate(ctx, probe.Prompt, GenOpts{Temperature: ContentTemperature, MaxTokens: ContentMaxTokens})
		if err != nil {
			core.Print(nil,"  [content:%s] ERROR: %v", probe.ID, err)
			continue
		}

		reply := StripThinkBlocks(res.Text)
		core.Print(nil,"  [content:%s] got %d chars", probe.ID, len(reply))

		responses = append(responses, ContentResponse{
			Probe:    probe,
			Response: reply,
		})
	}

	return responses
}

// RunContentProbesViaRunner sends content probes through an SSH probe runner.
func RunContentProbesViaRunner(stdin io.WriteCloser, scanner *bufio.Scanner) []ContentResponse {
	var responses []ContentResponse

	for _, probe := range ContentProbes {
		req := map[string]any{
			"prompt":     probe.Prompt,
			"max_tokens": ContentMaxTokens,
			"temp":       ContentTemperature,
		}
<<<<<<< HEAD
		reqJSON, _ := json.Marshal(req)
		core.Print(stdin, "%s", reqJSON)
=======
		reqJSON := core.JSONMarshalString(req)
		io.WriteString(stdin, core.Sprintf("%s\n", reqJSON))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

		var response string
		if scanner.Scan() {
			var resp probeRunnerResponse
<<<<<<< HEAD
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
				core.Print(nil,"  [content:%s] parse error: %v", probe.ID, err)
=======
			if r := core.JSONUnmarshalString(string(scanner.Bytes()), &resp); !r.OK {
				log.Printf("  [content:%s] parse error: %v", probe.ID, r.Value.(error))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
				continue
			} else if resp.Error != "" {
				core.Print(nil,"  [content:%s] ERROR: %s", probe.ID, resp.Error)
				continue
			} else {
				response = resp.Response
			}
		} else {
			core.Print(nil,"  [content:%s] no response from runner", probe.ID)
			continue
		}

		response = StripThinkBlocks(response)
		core.Print(nil,"  [content:%s] got %d chars", probe.ID, len(response))

		responses = append(responses, ContentResponse{
			Probe:    probe,
			Response: response,
		})
	}

	return responses
}
