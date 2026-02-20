package ml

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
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
		return fmt.Errorf("unknown Ollama model for tag %s", cp.ModelTag)
	}
	hfBase := HFBaseModelMap[cp.ModelTag]
	if hfBase == "" {
		hfBase = ollamaBase
	}

	tempModel := fmt.Sprintf("lem-%s-%d", cp.ModelTag, cp.Iteration)
	localAdapterDir := filepath.Join(cfg.WorkDir, "adapter-"+cp.Dirname)
	peftDir := filepath.Join(cfg.WorkDir, "peft-"+cp.Dirname)

	os.MkdirAll(localAdapterDir, 0755)

	defer func() {
		os.RemoveAll(localAdapterDir)
		os.RemoveAll(peftDir)
		OllamaDeleteModel(cfg.JudgeURL, tempModel)
	}()

	log.Printf("Fetching adapter from M3 (%s)...", cp.Filename)
	remoteSF := fmt.Sprintf("%s/%s", cp.RemoteDir, cp.Filename)
	remoteCfg := fmt.Sprintf("%s/adapter_config.json", cp.RemoteDir)
	localSF := filepath.Join(localAdapterDir, cp.Filename)
	localCfg := filepath.Join(localAdapterDir, "adapter_config.json")

	if err := SCPFrom(cfg, remoteSF, localSF); err != nil {
		return fmt.Errorf("scp safetensors: %w", err)
	}
	if err := SCPFrom(cfg, remoteCfg, localCfg); err != nil {
		return fmt.Errorf("scp config: %w", err)
	}

	log.Println("Converting MLX → PEFT format...")
	if err := ConvertMLXtoPEFT(localSF, localCfg, peftDir, hfBase); err != nil {
		return fmt.Errorf("convert adapter: %w", err)
	}

	log.Printf("Creating Ollama model %s (base: %s)...", tempModel, ollamaBase)
	if err := OllamaCreateModel(cfg.JudgeURL, tempModel, ollamaBase, peftDir); err != nil {
		return fmt.Errorf("ollama create: %w", err)
	}
	log.Printf("Ollama model %s ready", tempModel)

	ctx := context.Background()
	probeBackend := NewHTTPBackend(cfg.JudgeURL, tempModel)

	const baseTS int64 = 1739577600
	results, fullResponses := RunCapabilityProbesFull(ctx, probeBackend, func(probeID, category string, passed bool, response string, correct, total int) {
		passedInt := 0
		if passed {
			passedInt = 1
		}
		ts := (baseTS + int64(cp.Iteration)*1000 + int64(total+100)) * 1_000_000_000
		line := fmt.Sprintf(
			"probe_score,model=%s,run_id=%s,label=%s,probe_id=%s passed=%di,iteration=%di %d",
			EscapeLp(cp.ModelTag), EscapeLp(cp.RunID), EscapeLp(cp.Label), EscapeLp(probeID),
			passedInt, cp.Iteration, ts,
		)
		if err := influx.WriteLp([]string{line}); err != nil {
			log.Printf("  [%s] InfluxDB stream failed: %v", probeID, err)
		}
	})

	log.Printf("Capability: %s -- %.1f%% (%d/%d)",
		cp.Label, results.Accuracy, results.Correct, results.Total)

	if err := PushCapabilitySummary(influx, cp, results); err != nil {
		log.Printf("InfluxDB summary push failed, buffering: %v", err)
		BufferInfluxResult(cfg.WorkDir, cp, results)
	}
	PushCapabilityResultsDB(cfg.DBPath, cp, results)

	judgeBackend := NewHTTPBackend(cfg.JudgeURL, cfg.JudgeModel)
	judge := NewJudge(judgeBackend)

	log.Println("Judging 23 capability responses (0-10 quality scoring)...")
	ScoreCapabilityAndPush(ctx, judge, influx, cp, fullResponses)

	log.Println("Running 6 content probes (0-10 judge scoring)...")
	contentResponses := RunContentProbesViaAPI(ctx, probeBackend)
	if len(contentResponses) > 0 {
		contentRunID := strings.Replace(cp.RunID, "-capability-", "-content-", 1)
		ScoreContentAndPush(ctx, judge, influx, cp, contentRunID, contentResponses)
	}

	return nil
}

// processWithConversion fetches adapter locally, converts MLX→PEFT, and scores.
func processWithConversion(cfg *AgentConfig, influx *InfluxClient, cp Checkpoint) error {
	localAdapterDir := filepath.Join(cfg.WorkDir, cp.Dirname)
	os.MkdirAll(localAdapterDir, 0755)

	localSF := filepath.Join(localAdapterDir, cp.Filename)
	localCfg := filepath.Join(localAdapterDir, "adapter_config.json")

	defer func() {
		os.Remove(localSF)
		os.Remove(localCfg)
		peftDir := filepath.Join(cfg.WorkDir, fmt.Sprintf("peft_%07d", cp.Iteration))
		os.RemoveAll(peftDir)
	}()

	log.Println("Fetching adapter from M3...")
	remoteSF := fmt.Sprintf("%s/%s", cp.RemoteDir, cp.Filename)
	remoteCfg := fmt.Sprintf("%s/adapter_config.json", cp.RemoteDir)

	if err := SCPFrom(cfg, remoteSF, localSF); err != nil {
		return fmt.Errorf("scp safetensors: %w", err)
	}
	if err := SCPFrom(cfg, remoteCfg, localCfg); err != nil {
		return fmt.Errorf("scp config: %w", err)
	}

	log.Println("Converting MLX to PEFT format...")
	peftDir := filepath.Join(cfg.WorkDir, fmt.Sprintf("peft_%07d", cp.Iteration))
	if err := ConvertMLXtoPEFT(localSF, localCfg, peftDir, cfg.BaseModel); err != nil {
		return fmt.Errorf("convert adapter: %w", err)
	}

	log.Println("Running 23 capability probes...")
	ctx := context.Background()
	modelName := cfg.Model
	if modelName == "" {
		modelName = cp.ModelTag
	}
	backend := NewHTTPBackend(cfg.APIURL, modelName)

	results := RunCapabilityProbes(ctx, backend)

	log.Printf("Result: %s -- %.1f%% (%d/%d)",
		cp.Label, results.Accuracy, results.Correct, results.Total)

	if err := PushCapabilityResults(influx, cp, results); err != nil {
		log.Printf("InfluxDB push failed, buffering: %v", err)
		BufferInfluxResult(cfg.WorkDir, cp, results)
	}
	PushCapabilityResultsDB(cfg.DBPath, cp, results)

	return nil
}

// RunCapabilityProbes runs all 23 probes against a backend.
func RunCapabilityProbes(ctx context.Context, backend Backend) ProbeResult {
	results := ProbeResult{
		ByCategory: make(map[string]CategoryResult),
		Probes:     make(map[string]SingleProbeResult),
	}

	correct := 0
	total := 0

	for _, probe := range CapabilityProbes {
		response, err := backend.Generate(ctx, probe.Prompt, GenOpts{Temperature: 0.1, MaxTokens: 500})
		if err != nil {
			log.Printf("  [%s] ERROR: %v", probe.ID, err)
			results.Probes[probe.ID] = SingleProbeResult{Passed: false, Response: err.Error()}
			total++
			cat := results.ByCategory[probe.Category]
			cat.Total++
			results.ByCategory[probe.Category] = cat
			continue
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
		if len(stored) > 300 {
			stored = stored[:300]
		}
		results.Probes[probe.ID] = SingleProbeResult{Passed: passed, Response: stored}

		status := "FAIL"
		if passed {
			status = "PASS"
		}
		log.Printf("  [%s] %s (expected: %s)", probe.ID, status, probe.Answer)
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
		response, err := backend.Generate(ctx, probe.Prompt, GenOpts{Temperature: 0.1, MaxTokens: 500})
		if err != nil {
			log.Printf("  [%s] ERROR: %v", probe.ID, err)
			response = fmt.Sprintf("ERROR: %v", err)
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
		if len(stored) > 300 {
			stored = stored[:300]
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
		log.Printf("  [%s] %s (expected: %s)", probe.ID, status, probe.Answer)

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
		reply, err := backend.Generate(ctx, probe.Prompt, GenOpts{Temperature: 0.7, MaxTokens: 1000})
		if err != nil {
			log.Printf("  [content:%s] ERROR: %v", probe.ID, err)
			continue
		}

		reply = StripThinkBlocks(reply)
		log.Printf("  [content:%s] got %d chars", probe.ID, len(reply))

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
		req := map[string]interface{}{
			"prompt":     probe.Prompt,
			"max_tokens": 1000,
			"temp":       0.7,
		}
		reqJSON, _ := json.Marshal(req)
		fmt.Fprintf(stdin, "%s\n", reqJSON)

		var response string
		if scanner.Scan() {
			var resp probeRunnerResponse
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
				log.Printf("  [content:%s] parse error: %v", probe.ID, err)
				continue
			} else if resp.Error != "" {
				log.Printf("  [content:%s] ERROR: %s", probe.ID, resp.Error)
				continue
			} else {
				response = resp.Response
			}
		} else {
			log.Printf("  [content:%s] no response from runner", probe.ID)
			continue
		}

		response = StripThinkBlocks(response)
		log.Printf("  [content:%s] got %d chars", probe.ID, len(response))

		responses = append(responses, ContentResponse{
			Probe:    probe,
			Response: response,
		})
	}

	return responses
}
