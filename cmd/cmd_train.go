// TODO(virgil): Re-enable when go-mlx exports concrete model type for training.
// The old go-ai/mlx/model and go-ai/mlx/tokenizer packages were extracted to go-mlx
// but the training-specific API (LoadModel→concrete type with ApplyLoRA, Forward,
// NewCache, Tokenizer) is not yet re-exported through the public interface.
// See: https://forge.lthn.ai/core/go-mlx — needs training API surface.
//go:build ignore

package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"forge.lthn.ai/core/cli/pkg/cli"
	"forge.lthn.ai/core/go-ml"
	"forge.lthn.ai/core/go-mlx"
	"github.com/ollama/ollama/tokenizer"
)

var trainCmd = &cli.Command{
	Use:   "train",
	Short: "LoRA fine-tune a model on JSONL training data",
	Long: `Fine-tunes a local MLX model using LoRA (Low-Rank Adaptation).

Reads chat-format JSONL training data and trains LoRA adapter weights
using AdamW optimiser with cross-entropy loss on assistant tokens only.

Training data format (one JSON object per line):
  {"messages": [{"role": "system", "content": "..."}, {"role": "user", "content": "..."}, {"role": "assistant", "content": "..."}]}`,
	RunE: runTrain,
}

var (
	trainModelPath   string
	trainData        string
	trainOutput      string
	trainRank        int
	trainAlpha       float64
	trainLR          float64
	trainEpochs      int
	trainMaxSeqLen   int
	trainTargets     string
	trainMemoryLimit int
)

func init() {
	trainCmd.Flags().StringVar(&trainModelPath, "model-path", "", "Path to model directory (required)")
	trainCmd.Flags().StringVar(&trainData, "data", "", "Training JSONL file (required)")
	trainCmd.Flags().StringVar(&trainOutput, "output", "adapters.safetensors", "Output adapter file")
	trainCmd.Flags().IntVar(&trainRank, "rank", 8, "LoRA decomposition rank")
	trainCmd.Flags().Float64Var(&trainAlpha, "alpha", 16, "LoRA scaling factor")
	trainCmd.Flags().Float64Var(&trainLR, "lr", 1e-4, "Learning rate")
	trainCmd.Flags().IntVar(&trainEpochs, "epochs", 1, "Number of training epochs")
	trainCmd.Flags().IntVar(&trainMaxSeqLen, "max-seq-len", 512, "Maximum sequence length (tokens)")
	trainCmd.Flags().StringVar(&trainTargets, "targets", "q_proj,v_proj", "Comma-separated projection targets for LoRA")
	trainCmd.Flags().IntVar(&trainMemoryLimit, "memory-limit", 24, "Metal memory limit in GB")
	trainCmd.MarkFlagRequired("model-path")
	trainCmd.MarkFlagRequired("data")
}

// trainSample holds a tokenised training example.
type trainSample struct {
	Tokens []int32 // Full token sequence
	Mask   []int32 // 1 for assistant tokens, 0 for prompt tokens
}

func runTrain(cmd *cli.Command, args []string) error {
	start := time.Now()

	// --- Load model ---
	slog.Info("loading model", "path", trainModelPath)
	m, err := model.LoadModel(trainModelPath)
	if err != nil {
		return fmt.Errorf("load model: %w", err)
	}

	mlx.SetCacheLimit(uint64(trainMemoryLimit) * 1024 * 1024 * 1024)
	mlx.SetMemoryLimit(uint64(trainMemoryLimit) * 1024 * 1024 * 1024)

	tok := m.Tokenizer()
	slog.Info("model loaded",
		"type", m.ModelType(),
		"layers", m.NumLayers(),
	)

	// --- Apply LoRA ---
	targets := strings.Split(trainTargets, ",")
	cfg := mlx.LoRAConfig{
		Rank:       trainRank,
		Alpha:      float32(trainAlpha),
		TargetKeys: targets,
	}

	adapter := m.ApplyLoRA(cfg)
	slog.Info("LoRA applied",
		"rank", cfg.Rank,
		"alpha", cfg.Alpha,
		"targets", targets,
		"trainable_params", adapter.TotalParams(),
		"layers", len(adapter.Layers),
	)

	// --- Load training data ---
	samples, err := loadTrainingSamples(trainData, tok, m.ModelType(), trainMaxSeqLen)
	if err != nil {
		return fmt.Errorf("load training data: %w", err)
	}
	slog.Info("training data loaded", "samples", len(samples))

	if len(samples) == 0 {
		return errors.New("no training samples loaded")
	}

	// --- Training loop ---
	params := adapter.AllTrainableParams()
	opt := mlx.NewAdamW(trainLR)

	// Build argument indices for ValueAndGrad (all params)
	argIndices := make([]int, len(params))
	for i := range argIndices {
		argIndices[i] = i
	}

	var totalLoss float64
	var totalSteps int

	for epoch := 0; epoch < trainEpochs; epoch++ {
		var epochLoss float64
		epochStart := time.Now()

		for si, sample := range samples {
			// Build token tensors: input = tokens[:-1], target = tokens[1:]
			seqLen := len(sample.Tokens)
			if seqLen < 2 {
				continue
			}

			inputTokens := sample.Tokens[:seqLen-1]
			targetTokens := sample.Tokens[1:]
			maskTokens := sample.Mask[1:] // mask aligned with targets

			inputArr := mlx.FromValues(inputTokens, 1, len(inputTokens))
			targetArr := mlx.FromValues(targetTokens, 1, len(targetTokens))

			// Build float32 mask
			maskF32 := make([]float32, len(maskTokens))
			for i, m := range maskTokens {
				maskF32[i] = float32(m)
			}
			maskArr := mlx.FromValues(maskF32, 1, len(maskF32))
			mlx.Materialize(inputArr, targetArr, maskArr)

			// Loss function closure — takes LoRA params as inputs
			lossFn := func(inputs []*mlx.Array) []*mlx.Array {
				// Set LoRA params from inputs
				adapter.SetAllParams(inputs)

				// Forward pass with fresh caches (no KV caching for training)
				caches := m.NewCache()
				logits := m.Forward(inputArr, caches)

				// Cast targets to int32 for take_along_axis
				loss := mlx.MaskedCrossEntropyLoss(logits, targetArr, maskArr)
				return []*mlx.Array{loss}
			}

			// Compute value and gradients
			grad := mlx.ValueAndGrad(lossFn, argIndices...)
			values, grads, err := grad.Apply(params...)
			grad.Free()
			if err != nil {
				return fmt.Errorf("epoch %d sample %d: gradient failed: %w", epoch, si, err)
			}

			mlx.Materialize(append(values, grads...)...)

			loss := values[0].Float()
			epochLoss += loss
			totalSteps++

			// Update parameters
			params = opt.Step(params, grads)
			adapter.SetAllParams(params)
			mlx.Materialize(params...)

			// Periodic cleanup
			if totalSteps%4 == 0 {
				runtime.GC()
				mlx.ClearCache()
			}

			// Log progress
			if (si+1)%10 == 0 || si == len(samples)-1 {
				avgLoss := epochLoss / float64(si+1)
				slog.Info("training",
					"epoch", epoch+1,
					"step", fmt.Sprintf("%d/%d", si+1, len(samples)),
					"loss", fmt.Sprintf("%.4f", loss),
					"avg_loss", fmt.Sprintf("%.4f", avgLoss),
				)
			}
		}

		totalLoss = epochLoss / float64(len(samples))
		elapsed := time.Since(epochStart)
		slog.Info("epoch complete",
			"epoch", epoch+1,
			"avg_loss", fmt.Sprintf("%.4f", totalLoss),
			"duration", elapsed.Round(time.Second),
			"samples_per_sec", fmt.Sprintf("%.1f", float64(len(samples))/elapsed.Seconds()),
		)
	}

	// --- Save adapter ---
	if err := adapter.Save(trainOutput); err != nil {
		return fmt.Errorf("save adapter: %w", err)
	}

	slog.Info("training complete",
		"output", trainOutput,
		"total_steps", totalSteps,
		"final_loss", fmt.Sprintf("%.4f", totalLoss),
		"duration", time.Since(start).Round(time.Second),
		"trainable_params", adapter.TotalParams(),
	)

	return nil
}

// loadTrainingSamples reads JSONL and tokenises each conversation.
func loadTrainingSamples(path string, tok *tokenizer.Tokenizer, modelType string, maxSeqLen int) ([]trainSample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var samples []trainSample
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB line buffer

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var entry struct {
			Messages []ml.Message `json:"messages"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			slog.Warn("skipping invalid line", "line", lineNum, "error", err)
			continue
		}

		if len(entry.Messages) == 0 {
			continue
		}

		sample := tokeniseConversation(entry.Messages, tok, modelType, maxSeqLen)
		if sample != nil {
			samples = append(samples, *sample)
		}
	}

	return samples, scanner.Err()
}

// tokeniseConversation formats and tokenises a conversation, creating a mask
// that is 1 for assistant tokens and 0 for system/user tokens.
func tokeniseConversation(messages []ml.Message, tok *tokenizer.Tokenizer, modelType string, maxSeqLen int) *trainSample {
	// Strategy: tokenise the full conversation, then tokenise just the prefix
	// (non-assistant parts) to determine the mask boundary.

	// Build full conversation text
	fullText := formatConversation(messages, modelType, true)
	fullTokens := tok.Encode(fullText)

	if len(fullTokens) < 2 {
		return nil
	}

	// Truncate to max sequence length
	if len(fullTokens) > maxSeqLen {
		fullTokens = fullTokens[:maxSeqLen]
	}

	// Build mask: tokenise prefix (everything up to last assistant response)
	// then mark remaining tokens as assistant (mask=1)
	prefixText := formatConversation(messages, modelType, false)
	prefixTokens := tok.Encode(prefixText)

	mask := make([]int32, len(fullTokens))
	for i := range mask {
		if i >= len(prefixTokens) {
			mask[i] = 1 // assistant token
		}
	}

	return &trainSample{
		Tokens: fullTokens,
		Mask:   mask,
	}
}

// formatConversation formats messages using the model's chat template.
// If includeAssistant is false, only formats up to the last assistant turn header.
func formatConversation(messages []ml.Message, modelType string, includeAssistant bool) string {
	switch modelType {
	case "qwen3":
		return formatQwen3Train(messages, includeAssistant)
	default:
		return formatGemmaTrain(messages, includeAssistant)
	}
}

func formatQwen3Train(messages []ml.Message, includeAssistant bool) string {
	var sb strings.Builder
	for _, msg := range messages {
		if msg.Role == "assistant" && !includeAssistant {
			// Write the assistant header but not the content
			sb.WriteString("<|im_start|>assistant\n")
			return sb.String()
		}
		switch msg.Role {
		case "system":
			sb.WriteString(fmt.Sprintf("<|im_start|>system\n%s<|im_end|>\n", msg.Content))
		case "user":
			sb.WriteString(fmt.Sprintf("<|im_start|>user\n%s<|im_end|>\n", msg.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("<|im_start|>assistant\n%s<|im_end|>\n", msg.Content))
		}
	}
	return sb.String()
}

func formatGemmaTrain(messages []ml.Message, includeAssistant bool) string {
	var sb strings.Builder
	for _, msg := range messages {
		if msg.Role == "assistant" && !includeAssistant {
			sb.WriteString("<start_of_turn>model\n")
			return sb.String()
		}
		switch msg.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("<start_of_turn>user\n%s<end_of_turn>\n", msg.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("<start_of_turn>model\n%s<end_of_turn>\n", msg.Content))
		case "system":
			sb.WriteString(fmt.Sprintf("<start_of_turn>user\n[System: %s]<end_of_turn>\n", msg.Content))
		}
	}
	return sb.String()
}
