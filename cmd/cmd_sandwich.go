//go:build darwin && arm64 && !nomlx

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"dappco.re/go/core/cli/pkg/cli"
)

var sandwichCmd = &cli.Command{
	Use:   "sandwich",
	Short: "Generate LEK training data using sandwich signing",
	Long: `Generates training data by wrapping seed prompts in a "sandwich" format:

  KB preamble (axioms framework) → seed prompt → LEK-1 kernel postfix

Each seed prompt is sent to the local MLX model for inference, and the
signed prompt + response pair is written as chat JSONL for 'core ml train'.

The "sandwich" format embeds the ethical framework context around each
prompt, teaching the model to reason from LEK principles naturally.

Seed file format (JSON array):
  [{"id": "P01", "category": "sovereignty", "prompt": "...", "signal": "..."}]`,
	RunE: runSandwich,
}

var (
	sandwichModelPath string
	sandwichKB        string
	sandwichKernel    string
	sandwichSeeds     string
	sandwichOutput    string
	sandwichMaxTokens int
	sandwichTemp      float64
	sandwichMemLimit  int
	sandwichDryRun    bool
)

func init() {
	sandwichCmd.Flags().StringVar(&sandwichModelPath, "model-path", "", "Path to model directory (required)")
	sandwichCmd.Flags().StringVar(&sandwichKB, "kb", "", "Knowledge base document (axioms markdown, required)")
	sandwichCmd.Flags().StringVar(&sandwichKernel, "kernel", "", "LEK-1 kernel file (required)")
	sandwichCmd.Flags().StringVar(&sandwichSeeds, "seeds", "", "Seed prompts JSON file (required)")
	sandwichCmd.Flags().StringVar(&sandwichOutput, "output", "sandwich.jsonl", "Output JSONL file")
	sandwichCmd.Flags().IntVar(&sandwichMaxTokens, "max-tokens", 1024, "Max tokens per response")
	sandwichCmd.Flags().Float64Var(&sandwichTemp, "temperature", 0.4, "Sampling temperature")
	sandwichCmd.Flags().IntVar(&sandwichMemLimit, "memory-limit", 24, "Metal memory limit in GB")
	sandwichCmd.Flags().BoolVar(&sandwichDryRun, "dry-run", false, "Output prompts only (no inference)")
	sandwichCmd.MarkFlagRequired("model-path")
	sandwichCmd.MarkFlagRequired("kernel")
	sandwichCmd.MarkFlagRequired("seeds")
	sandwichCmd.MarkFlagRequired("kb")
}

// seedPrompt is a single prompt from the seeds JSON file.
type seedPrompt struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Prompt   string `json:"prompt"`
	Signal   string `json:"signal"`
}

// sandwichOutput holds a single training example in messages format.
type sandwichRecord struct {
	Messages []ml.Message `json:"messages"`
}

func runSandwich(cmd *cli.Command, args []string) error {
	start := time.Now()

	// Load KB document
	kbText, err := coreio.Local.Read(sandwichKB)
	if err != nil {
		return coreerr.E("cmd.runSandwich", "read KB", err)
	}

	// Load LEK-1 kernel
	kernelText, err := coreio.Local.Read(sandwichKernel)
	if err != nil {
		return coreerr.E("cmd.runSandwich", "read kernel", err)
	}

	// Load seed prompts
	seedBytes, err := coreio.Local.Read(sandwichSeeds)
	if err != nil {
		return coreerr.E("cmd.runSandwich", "read seeds", err)
	}
	var seeds []seedPrompt
	if err := json.Unmarshal([]byte(seedBytes), &seeds); err != nil {
		return coreerr.E("cmd.runSandwich", "parse seeds", err)
	}

	slog.Info("sandwich: loaded inputs",
		"kb_chars", len(kbText),
		"kernel_chars", len(kernelText),
		"seeds", len(seeds),
	)

	if len(seeds) == 0 {
		return coreerr.E("cmd.runSandwich", "no seed prompts found", nil)
	}

	// Open output file
	outFile, err := coreio.Local.Create(sandwichOutput)
	if err != nil {
		return coreerr.E("cmd.runSandwich", "create output", err)
	}
	defer outFile.Close()
	encoder := json.NewEncoder(outFile)

	// Dry-run mode: output prompts without inference
	if sandwichDryRun {
		for _, seed := range seeds {
			signedPrompt := buildSandwich(kbText, seed.Prompt, kernelText)
			record := sandwichRecord{
				Messages: []ml.Message{
					{Role: "user", Content: signedPrompt},
				},
			}
			if err := encoder.Encode(record); err != nil {
				return coreerr.E("cmd.runSandwich", "write record", err)
			}
		}
		slog.Info("sandwich: dry-run complete",
			"output", sandwichOutput,
			"prompts", len(seeds),
		)
		return nil
	}

	// Load MLX model
	slog.Info("sandwich: loading model", "path", sandwichModelPath)
	backend, err := ml.NewMLXBackend(sandwichModelPath)
	if err != nil {
		return coreerr.E("cmd.runSandwich", "load model", err)
	}

	opts := ml.GenOpts{
		Temperature: sandwichTemp,
		MaxTokens:   sandwichMaxTokens,
	}

	var totalTokenTime time.Duration
	generated := 0

	for i, seed := range seeds {
		seedStart := time.Now()

		// Build the sandwich: KB + prompt + kernel
		signedPrompt := buildSandwich(kbText, seed.Prompt, kernelText)

		// Send as a user message for chat-style generation
		messages := []ml.Message{
			{Role: "user", Content: signedPrompt},
		}

		slog.Info("sandwich: generating",
			"seed", fmt.Sprintf("%d/%d", i+1, len(seeds)),
			"id", seed.ID,
			"category", seed.Category,
		)

		// Generate response
		res, err := backend.Chat(context.Background(), messages, opts)
		if err != nil {
			slog.Error("sandwich: generation failed",
				"id", seed.ID,
				"error", err,
			)
			continue
		}

		response := res.Text
		elapsed := time.Since(seedStart)
		totalTokenTime += elapsed

		// Write training record
		record := sandwichRecord{
			Messages: []ml.Message{
				{Role: "user", Content: signedPrompt},
				{Role: "assistant", Content: response},
			},
		}
		if err := encoder.Encode(record); err != nil {
			return coreerr.E("cmd.runSandwich", "write record", err)
		}

		generated++
		slog.Info("sandwich: generated",
			"id", seed.ID,
			"category", seed.Category,
			"response_chars", len(response),
			"duration", elapsed.Round(time.Second),
		)

		// Periodic cleanup
		if (i+1)%4 == 0 {
			runtime.GC()
		}
	}

	slog.Info("sandwich: complete",
		"output", sandwichOutput,
		"generated", generated,
		"total", len(seeds),
		"duration", time.Since(start).Round(time.Second),
		"avg_per_seed", (totalTokenTime / time.Duration(max(generated, 1))).Round(time.Second),
	)

	return nil
}

// buildSandwich constructs the signed prompt: KB preamble + seed prompt + LEK-1 kernel.
func buildSandwich(kb, prompt, kernel string) string {
	return fmt.Sprintf(`Name: Ethics Experiment
KB:
%s

---

%s

---

%s

Remember: respond using the ethical framework above. Do not reference the framework directly — reason from its principles naturally.`, kb, prompt, kernel)
}
