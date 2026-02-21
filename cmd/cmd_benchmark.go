//go:build darwin && arm64

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sort"
	"time"

	"forge.lthn.ai/core/go-ml"
	"forge.lthn.ai/core/go/pkg/cli"
)

var benchmarkCmd = &cli.Command{
	Use:   "benchmark",
	Short: "Compare baseline vs fine-tuned model on ethics probes",
	Long: `Runs the same prompts through a baseline model and a fine-tuned model,
scores both using the heuristic scorer, and outputs a comparison.

Uses the built-in LEK content probes by default. Optionally takes a
custom prompts JSONL file (same format as 'core ml score --input').

The fine-tuned model can be the same model directory with a LoRA adapter
loaded, or a separately merged model.`,
	RunE: runBenchmark,
}

var (
	benchmarkBaseline  string
	benchmarkTrained   string
	benchmarkPrompts   string
	benchmarkOutput    string
	benchmarkMaxTokens int
	benchmarkTemp      float64
	benchmarkMemLimit  int
)

func init() {
	benchmarkCmd.Flags().StringVar(&benchmarkBaseline, "baseline", "", "Path to baseline model directory (required)")
	benchmarkCmd.Flags().StringVar(&benchmarkTrained, "trained", "", "Path to fine-tuned model directory (required)")
	benchmarkCmd.Flags().StringVar(&benchmarkPrompts, "prompts", "", "Custom prompts file (JSONL with 'prompt' field, or seeds JSON)")
	benchmarkCmd.Flags().StringVar(&benchmarkOutput, "output", "benchmark.json", "Output comparison JSON file")
	benchmarkCmd.Flags().IntVar(&benchmarkMaxTokens, "max-tokens", 1024, "Max tokens per response")
	benchmarkCmd.Flags().Float64Var(&benchmarkTemp, "temperature", 0.4, "Sampling temperature")
	benchmarkCmd.Flags().IntVar(&benchmarkMemLimit, "memory-limit", 24, "Metal memory limit in GB")
	benchmarkCmd.MarkFlagRequired("baseline")
	benchmarkCmd.MarkFlagRequired("trained")
}

// benchmarkResult holds the comparison for a single prompt.
type benchmarkResult struct {
	ID               string  `json:"id"`
	Prompt           string  `json:"prompt"`
	BaselineResponse string  `json:"baseline_response"`
	TrainedResponse  string  `json:"trained_response"`
	BaselineLEK      float64 `json:"baseline_lek_score"`
	TrainedLEK       float64 `json:"trained_lek_score"`
	Delta            float64 `json:"delta"`

	BaselineHeuristic *ml.HeuristicScores `json:"baseline_heuristic"`
	TrainedHeuristic  *ml.HeuristicScores `json:"trained_heuristic"`
}

// benchmarkSummary holds aggregate comparison metrics.
type benchmarkSummary struct {
	BaselineModel   string             `json:"baseline_model"`
	TrainedModel    string             `json:"trained_model"`
	TotalPrompts    int                `json:"total_prompts"`
	AvgBaselineLEK  float64            `json:"avg_baseline_lek"`
	AvgTrainedLEK   float64            `json:"avg_trained_lek"`
	AvgDelta        float64            `json:"avg_delta"`
	Improved        int                `json:"improved"`
	Regressed       int                `json:"regressed"`
	Unchanged       int                `json:"unchanged"`
	Duration        string             `json:"duration"`
	Results         []benchmarkResult  `json:"results"`
}

func runBenchmark(cmd *cli.Command, args []string) error {
	start := time.Now()

	// Load prompts — either custom file or built-in probes
	prompts, err := loadBenchmarkPrompts()
	if err != nil {
		return err
	}

	slog.Info("benchmark: loaded prompts", "count", len(prompts))

	opts := ml.GenOpts{
		Temperature: benchmarkTemp,
		MaxTokens:   benchmarkMaxTokens,
	}

	// Generate baseline responses
	slog.Info("benchmark: loading baseline model", "path", benchmarkBaseline)
	baselineBackend, err := ml.NewMLXBackend(benchmarkBaseline)
	if err != nil {
		return fmt.Errorf("load baseline: %w", err)
	}

	baselineResponses := make(map[string]string)
	for i, p := range prompts {
		slog.Info("benchmark: baseline",
			"prompt", fmt.Sprintf("%d/%d", i+1, len(prompts)),
			"id", p.id,
		)
		resp, err := baselineBackend.Generate(context.Background(), p.prompt, opts)
		if err != nil {
			slog.Error("benchmark: baseline failed", "id", p.id, "error", err)
			continue
		}
		baselineResponses[p.id] = resp

		if (i+1)%4 == 0 {
			runtime.GC()
		}
	}

	// Force cleanup before loading second model
	baselineBackend = nil
	runtime.GC()
	runtime.GC()

	// Generate trained responses
	slog.Info("benchmark: loading trained model", "path", benchmarkTrained)
	trainedBackend, err := ml.NewMLXBackend(benchmarkTrained)
	if err != nil {
		return fmt.Errorf("load trained: %w", err)
	}

	trainedResponses := make(map[string]string)
	for i, p := range prompts {
		slog.Info("benchmark: trained",
			"prompt", fmt.Sprintf("%d/%d", i+1, len(prompts)),
			"id", p.id,
		)
		resp, err := trainedBackend.Generate(context.Background(), p.prompt, opts)
		if err != nil {
			slog.Error("benchmark: trained failed", "id", p.id, "error", err)
			continue
		}
		trainedResponses[p.id] = resp

		if (i+1)%4 == 0 {
			runtime.GC()
		}
	}

	trainedBackend = nil
	runtime.GC()

	// Score both sets
	var results []benchmarkResult
	var totalBaseline, totalTrained float64
	improved, regressed, unchanged := 0, 0, 0

	for _, p := range prompts {
		baseResp := baselineResponses[p.id]
		trainResp := trainedResponses[p.id]

		if baseResp == "" || trainResp == "" {
			continue
		}

		baseH := ml.ScoreHeuristic(baseResp)
		trainH := ml.ScoreHeuristic(trainResp)
		delta := trainH.LEKScore - baseH.LEKScore

		totalBaseline += baseH.LEKScore
		totalTrained += trainH.LEKScore

		if delta > 0.5 {
			improved++
		} else if delta < -0.5 {
			regressed++
		} else {
			unchanged++
		}

		results = append(results, benchmarkResult{
			ID:                p.id,
			Prompt:            p.prompt,
			BaselineResponse:  baseResp,
			TrainedResponse:   trainResp,
			BaselineLEK:       baseH.LEKScore,
			TrainedLEK:        trainH.LEKScore,
			Delta:             delta,
			BaselineHeuristic: baseH,
			TrainedHeuristic:  trainH,
		})
	}

	n := float64(len(results))
	if n == 0 {
		return fmt.Errorf("no results to compare")
	}

	summary := benchmarkSummary{
		BaselineModel:  benchmarkBaseline,
		TrainedModel:   benchmarkTrained,
		TotalPrompts:   len(results),
		AvgBaselineLEK: totalBaseline / n,
		AvgTrainedLEK:  totalTrained / n,
		AvgDelta:       (totalTrained - totalBaseline) / n,
		Improved:       improved,
		Regressed:      regressed,
		Unchanged:      unchanged,
		Duration:       time.Since(start).Round(time.Second).String(),
		Results:        results,
	}

	// Write output
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	if err := os.WriteFile(benchmarkOutput, data, 0644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	// Print summary
	fmt.Println()
	fmt.Println("=== Benchmark Results ===")
	fmt.Printf("Baseline:  %s\n", benchmarkBaseline)
	fmt.Printf("Trained:   %s\n", benchmarkTrained)
	fmt.Printf("Prompts:   %d\n", len(results))
	fmt.Println()
	fmt.Printf("Avg LEK (baseline): %+.2f\n", summary.AvgBaselineLEK)
	fmt.Printf("Avg LEK (trained):  %+.2f\n", summary.AvgTrainedLEK)
	fmt.Printf("Avg Delta:          %+.2f\n", summary.AvgDelta)
	fmt.Println()
	fmt.Printf("Improved:   %d (%.0f%%)\n", improved, float64(improved)/n*100)
	fmt.Printf("Regressed:  %d (%.0f%%)\n", regressed, float64(regressed)/n*100)
	fmt.Printf("Unchanged:  %d (%.0f%%)\n", unchanged, float64(unchanged)/n*100)
	fmt.Printf("Duration:   %s\n", summary.Duration)
	fmt.Printf("Output:     %s\n", benchmarkOutput)

	return nil
}

type benchPrompt struct {
	id     string
	prompt string
}

func loadBenchmarkPrompts() ([]benchPrompt, error) {
	if benchmarkPrompts == "" {
		// Use built-in content probes
		probes := ml.ContentProbes
		prompts := make([]benchPrompt, len(probes))
		for i, p := range probes {
			prompts[i] = benchPrompt{id: p.ID, prompt: p.Prompt}
		}
		return prompts, nil
	}

	// Try seeds JSON format first (array of {id, prompt, ...})
	data, err := os.ReadFile(benchmarkPrompts)
	if err != nil {
		return nil, fmt.Errorf("read prompts: %w", err)
	}

	var seeds []seedPrompt
	if json.Unmarshal(data, &seeds) == nil && len(seeds) > 0 {
		prompts := make([]benchPrompt, len(seeds))
		for i, s := range seeds {
			prompts[i] = benchPrompt{id: s.ID, prompt: s.Prompt}
		}
		return prompts, nil
	}

	// Try JSONL responses format
	responses, err := ml.ReadResponses(benchmarkPrompts)
	if err != nil {
		return nil, fmt.Errorf("parse prompts: %w", err)
	}

	// Deduplicate by prompt
	seen := make(map[string]bool)
	var prompts []benchPrompt
	for _, r := range responses {
		if seen[r.Prompt] {
			continue
		}
		seen[r.Prompt] = true
		id := r.ID
		if id == "" {
			id = fmt.Sprintf("P%03d", len(prompts)+1)
		}
		prompts = append(prompts, benchPrompt{id: id, prompt: r.Prompt})
	}

	sort.Slice(prompts, func(i, j int) bool { return prompts[i].id < prompts[j].id })
	return prompts, nil
}
