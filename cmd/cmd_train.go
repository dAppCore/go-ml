//go:build darwin && arm64

package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"forge.lthn.ai/core/cli/pkg/cli"
	ml "forge.lthn.ai/core/go-ml"

	"forge.lthn.ai/core/go-inference"
	"forge.lthn.ai/core/go-mlx"
)

var trainCmd = &cli.Command{
	Use:   "train",
	Short: "LoRA fine-tune a model on JSONL training data",
	Long: `Fine-tunes a local MLX model using LoRA (Low-Rank Adaptation).

Reads chat-format JSONL training data and trains LoRA adapter weights
using AdamW optimiser with cross-entropy loss on assistant tokens only.

Features:
  - Cosine learning rate schedule with warmup
  - Periodic validation loss evaluation
  - Checkpoint saves with adapter_config.json
  - InfluxDB telemetry (loss, perplexity, tokens/sec, peak memory)
  - Live scoring via InfluxDB queue (homelab runner picks up jobs)
  - Gradient checkpointing for memory efficiency
  - 90/10 train/valid split

Training data format (one JSON object per line):
  {"messages": [{"role": "user", "content": "..."}, {"role": "assistant", "content": "..."}]}`,
	RunE: runTrain,
}

var (
	trainModelPath    string
	trainData         string
	trainOutputDir    string
	trainRank         int
	trainAlpha        float64
	trainLR           float64
	trainMinLR        float64
	trainEpochs       int
	trainIters        int
	trainMaxSeqLen    int
	trainTargets      string
	trainMemoryLimit  int
	trainCheckEvery   int
	trainValEvery     int
	trainScoreEvery   int
	trainLogEvery     int
	trainValidSplit   float64
	trainRunID        string
	trainPhase        string
	trainNoTelemetry    bool
	trainGradCheckpoint bool
	trainNoTUI          bool
)

func init() {
	trainCmd.Flags().StringVar(&trainModelPath, "model-path", "", "Path to model directory (required)")
	trainCmd.Flags().StringVar(&trainData, "data", "", "Training JSONL file (required)")
	trainCmd.Flags().StringVar(&trainOutputDir, "output", "./adapters", "Output directory for adapter files")
	trainCmd.Flags().IntVar(&trainRank, "rank", 16, "LoRA decomposition rank")
	trainCmd.Flags().Float64Var(&trainAlpha, "alpha", 32, "LoRA scaling factor")
	trainCmd.Flags().Float64Var(&trainLR, "lr", 2e-5, "Peak learning rate")
	trainCmd.Flags().Float64Var(&trainMinLR, "min-lr", 1e-6, "Minimum learning rate (cosine floor)")
	trainCmd.Flags().IntVar(&trainEpochs, "epochs", 0, "Number of training epochs (0 = use --iters)")
	trainCmd.Flags().IntVar(&trainIters, "iters", 400, "Number of training iterations (ignored if --epochs > 0)")
	trainCmd.Flags().IntVar(&trainMaxSeqLen, "max-seq-len", 3072, "Maximum sequence length (tokens)")
	trainCmd.Flags().StringVar(&trainTargets, "targets", "q_proj,k_proj,v_proj,o_proj,gate_proj,up_proj,down_proj", "Comma-separated projection targets for LoRA")
	trainCmd.Flags().IntVar(&trainMemoryLimit, "memory-limit", 48, "Metal memory limit in GB")
	trainCmd.Flags().IntVar(&trainCheckEvery, "checkpoint-every", 100, "Save checkpoint every N iterations (0 = off)")
	trainCmd.Flags().IntVar(&trainValEvery, "val-every", 50, "Evaluate validation loss every N iterations (0 = off)")
	trainCmd.Flags().IntVar(&trainScoreEvery, "score-every", 25, "Queue live response for scoring every N iterations (0 = off)")
	trainCmd.Flags().IntVar(&trainLogEvery, "log-every", 10, "Log training metrics every N iterations")
	trainCmd.Flags().Float64Var(&trainValidSplit, "valid-split", 0.1, "Fraction of data for validation (0 = no validation)")
	trainCmd.Flags().StringVar(&trainRunID, "run-id", "", "Run ID for telemetry (auto-generated if empty)")
	trainCmd.Flags().StringVar(&trainPhase, "phase", "P0", "Training phase tag for telemetry")
	trainCmd.Flags().BoolVar(&trainNoTelemetry, "no-telemetry", false, "Disable InfluxDB telemetry")
	trainCmd.Flags().BoolVar(&trainGradCheckpoint, "grad-checkpoint", true, "Enable gradient checkpointing (saves memory)")
	trainCmd.Flags().BoolVar(&trainNoTUI, "no-tui", false, "Disable TUI dashboard (log to stdout instead)")
	trainCmd.MarkFlagRequired("model-path")
	trainCmd.MarkFlagRequired("data")
}

// trainSample holds a tokenised training example.
type trainSample struct {
	Tokens []int32
	Mask   []int32 // 1 for assistant tokens, 0 for prompt tokens
}

// cosineDecay returns a learning rate for the given step using cosine annealing.
func cosineDecay(step, totalSteps int, maxLR, minLR float64) float64 {
	if step >= totalSteps {
		return minLR
	}
	progress := float64(step) / float64(totalSteps)
	return minLR + 0.5*(maxLR-minLR)*(1+math.Cos(math.Pi*progress))
}

func runTrain(cobraCmd *cli.Command, args []string) error {
	// --- TUI mode ---
	if !trainNoTUI {
		return runTrainTUI(cobraCmd, args)
	}
	return runTrainLoop(cobraCmd, nil)
}

// runTrainTUI launches the TUI dashboard and runs training in a background goroutine.
func runTrainTUI(cobraCmd *cli.Command, _ []string) error {
	tui := NewTrainFrame()

	// Run training in background, send ticks to TUI
	var trainErr error
	go func() {
		trainErr = runTrainLoop(cobraCmd, tui)
		tui.SendDone(trainErr)
	}()

	tui.Run()
	return trainErr
}

// runTrainLoop is the core training loop. If tui is non-nil, it sends progress ticks.
func runTrainLoop(cobraCmd *cli.Command, tui *TrainFrame) error {
	start := time.Now()

	// --- Auto-generate run ID ---
	if trainRunID == "" {
		trainRunID = fmt.Sprintf("train-%s", time.Now().Format("20060102-150405"))
	}

	// --- Load model ---
	slog.Info("loading model", "path", trainModelPath)
	tm, err := inference.LoadTrainable(trainModelPath)
	if err != nil {
		return fmt.Errorf("load model: %w", err)
	}
	defer tm.Close()

	mlx.SetCacheLimit(uint64(trainMemoryLimit) * 1024 * 1024 * 1024)
	mlx.SetMemoryLimit(uint64(trainMemoryLimit) * 1024 * 1024 * 1024)

	slog.Info("model loaded",
		"type", tm.ModelType(),
		"layers", tm.NumLayers(),
	)

	// --- Apply LoRA ---
	targets := strings.Split(trainTargets, ",")
	cfg := inference.LoRAConfig{
		Rank:       trainRank,
		Alpha:      float32(trainAlpha),
		TargetKeys: targets,
	}

	adapter := tm.ApplyLoRA(cfg)
	loraAdapter := mlx.ConcreteAdapter(adapter)
	slog.Info("LoRA applied",
		"rank", cfg.Rank,
		"alpha", cfg.Alpha,
		"targets", targets,
		"trainable_params", adapter.TotalParams(),
		"layers", len(loraAdapter.Layers),
	)

	// --- Load and split training data ---
	allSamples, err := loadTrainingSamples(trainData, tm, trainMaxSeqLen)
	if err != nil {
		return fmt.Errorf("load training data: %w", err)
	}
	if len(allSamples) == 0 {
		return errors.New("no training samples loaded")
	}

	// Split into train/valid
	splitIdx := len(allSamples)
	var validSamples []trainSample
	if trainValidSplit > 0 && trainValidSplit < 1 {
		splitIdx = int(float64(len(allSamples)) * (1 - trainValidSplit))
		if splitIdx < 1 {
			splitIdx = 1
		}
		validSamples = allSamples[splitIdx:]
	}
	trainSamples := allSamples[:splitIdx]

	slog.Info("training data loaded",
		"total", len(allSamples),
		"train", len(trainSamples),
		"valid", len(validSamples),
	)

	// --- Output directory ---
	if err := os.MkdirAll(trainOutputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	adapterFile := filepath.Join(trainOutputDir, "adapters.safetensors")

	// --- Telemetry ---
	var influx *ml.InfluxClient
	if !trainNoTelemetry {
		influx = ml.NewInfluxClient("", "")
	}
	modelTag := ml.EscapeLp(filepath.Base(trainModelPath))

	// --- Determine total iterations ---
	totalIters := trainIters
	if trainEpochs > 0 {
		totalIters = trainEpochs * len(trainSamples)
	}

	// --- Get concrete Metal model for training loop ---
	internal := mlx.TrainingModel(tm)

	// --- Training loop ---
	params := loraAdapter.AllTrainableParams()
	opt := mlx.NewAdamW(trainLR)

	argIndices := make([]int, len(params))
	for i := range argIndices {
		argIndices[i] = i
	}

	var (
		trainedTokens int64
		runningLoss   float64
		runningCount  int
		lastLogTime   = time.Now()
		lastLogTokens int64
	)

	slog.Info("starting training",
		"iters", totalIters,
		"lr", trainLR,
		"min_lr", trainMinLR,
		"seq_len", trainMaxSeqLen,
		"run_id", trainRunID,
		"phase", trainPhase,
		"grad_checkpoint", trainGradCheckpoint,
	)

	for it := 1; it <= totalIters; it++ {
		// Cycle through training samples
		sample := trainSamples[(it-1)%len(trainSamples)]

		seqLen := len(sample.Tokens)
		if seqLen < 2 {
			continue
		}

		// Cosine learning rate
		lr := cosineDecay(it-1, totalIters, trainLR, trainMinLR)
		opt.LR = lr

		inputTokens := sample.Tokens[:seqLen-1]
		targetTokens := sample.Tokens[1:]
		maskTokens := sample.Mask[1:]

		inputArr := mlx.FromValues(inputTokens, 1, len(inputTokens))
		targetArr := mlx.FromValues(targetTokens, 1, len(targetTokens))

		maskF32 := make([]float32, len(maskTokens))
		for i, m := range maskTokens {
			maskF32[i] = float32(m)
		}
		maskArr := mlx.FromValues(maskF32, 1, len(maskF32))
		mlx.Materialize(inputArr, targetArr, maskArr)

		// Loss function closure
		lossFn := func(inputs []*mlx.Array) []*mlx.Array {
			loraAdapter.SetAllParams(inputs)
			caches := internal.NewCache()
			logits := internal.Forward(inputArr, caches)
			loss := mlx.MaskedCrossEntropyLoss(logits, targetArr, maskArr)
			return []*mlx.Array{loss}
		}

		// Optionally wrap in gradient checkpoint
		actualLossFn := lossFn
		if trainGradCheckpoint {
			actualLossFn = mlx.Checkpoint(lossFn)
		}

		// Compute value and gradients
		grad := mlx.ValueAndGrad(actualLossFn, argIndices...)
		values, grads, err := grad.Apply(params...)
		grad.Free()
		if err != nil {
			return fmt.Errorf("iter %d: gradient failed: %w", it, err)
		}

		mlx.Materialize(append(values, grads...)...)

		loss := values[0].Float()
		runningLoss += loss
		runningCount++
		trainedTokens += int64(len(inputTokens))

		// Update parameters
		params = opt.Step(params, grads)
		loraAdapter.SetAllParams(params)
		mlx.Materialize(params...)

		// Periodic cleanup
		if it%5 == 0 {
			mlx.ClearCache()
		}
		if it%20 == 0 {
			runtime.GC()
		}

		// --- Log training metrics ---
		if trainLogEvery > 0 && it%trainLogEvery == 0 {
			avgLoss := runningLoss / float64(runningCount)
			peak := float64(mlx.GetPeakMemory()) / 1e9
			now := time.Now()
			dt := now.Sub(lastLogTime).Seconds()
			dtok := trainedTokens - lastLogTokens
			tps := float64(dtok) / dt
			perplexity := math.Exp(math.Min(avgLoss, 20))

			if tui == nil {
				slog.Info("train",
					"iter", fmt.Sprintf("%d/%d", it, totalIters),
					"loss", fmt.Sprintf("%.4f", avgLoss),
					"ppl", fmt.Sprintf("%.2f", perplexity),
					"lr", fmt.Sprintf("%.2e", lr),
					"tok/s", fmt.Sprintf("%.0f", tps),
					"peak", fmt.Sprintf("%.1fGB", peak),
					"tokens", trainedTokens,
				)
			} else {
				tui.SendTick(it, totalIters, avgLoss, 0, lr, tps, peak, trainedTokens, trainPhase, trainRunID)
			}

			// InfluxDB telemetry
			if influx != nil {
				fields := fmt.Sprintf(
					"loss=%.6f,perplexity=%.4f,tokens_total=%di,tokens_per_sec=%.1f,peak_memory_gb=%.2f,iteration=%di,lr=%.8f",
					avgLoss, perplexity, trainedTokens, tps, peak, it, lr,
				)
				lp := fmt.Sprintf("training_loss,model=%s,run_id=%s,phase=%s,loss_type=train %s",
					modelTag, ml.EscapeLp(trainRunID), ml.EscapeLp(trainPhase), fields)
				_ = influx.WriteLp([]string{lp})
			}

			runningLoss = 0
			runningCount = 0
			lastLogTime = now
			lastLogTokens = trainedTokens
		}

		// --- Validation loss ---
		if trainValEvery > 0 && len(validSamples) > 0 && it%trainValEvery == 0 {
			valLoss := evalValidation(internal, loraAdapter, validSamples, trainMaxSeqLen)

			if tui == nil {
				slog.Info("validation",
					"iter", it,
					"val_loss", fmt.Sprintf("%.4f", valLoss),
					"val_ppl", fmt.Sprintf("%.2f", math.Exp(math.Min(valLoss, 20))),
				)
			} else {
				peak := float64(mlx.GetPeakMemory()) / 1e9
				tui.SendTick(it, totalIters, 0, valLoss, lr, 0, peak, trainedTokens, trainPhase, trainRunID)
			}

			if influx != nil {
				lp := fmt.Sprintf("training_loss,model=%s,run_id=%s,phase=%s,loss_type=val loss=%.6f,iteration=%di",
					modelTag, ml.EscapeLp(trainRunID), ml.EscapeLp(trainPhase), valLoss, it)
				_ = influx.WriteLp([]string{lp})
			}
			mlx.ClearCache()
		}

		// --- Queue live response for scoring ---
		if trainScoreEvery > 0 && influx != nil && it%trainScoreEvery == 0 {
			queueLiveScore(cobraCmd.Context(), tm, trainSamples, it, influx, modelTag)
			mlx.ClearCache()
		}

		// --- Checkpoint ---
		if trainCheckEvery > 0 && it%trainCheckEvery == 0 {
			if err := saveCheckpoint(adapter, trainOutputDir, it, cfg); err != nil {
				slog.Warn("checkpoint save failed", "iter", it, "error", err)
			} else {
				slog.Info("checkpoint saved", "iter", it)
			}
			mlx.ClearCache()
		}
	}

	// --- Final save ---
	if err := adapter.Save(adapterFile); err != nil {
		return fmt.Errorf("save final adapter: %w", err)
	}
	if err := writeAdapterConfig(trainOutputDir, cfg); err != nil {
		slog.Warn("write adapter_config.json failed", "error", err)
	}

	elapsed := time.Since(start)
	avgLoss := 0.0
	if runningCount > 0 {
		avgLoss = runningLoss / float64(runningCount)
	}

	slog.Info("training complete",
		"output", adapterFile,
		"total_iters", totalIters,
		"final_loss", fmt.Sprintf("%.4f", avgLoss),
		"total_tokens", trainedTokens,
		"duration", elapsed.Round(time.Second),
		"trainable_params", adapter.TotalParams(),
	)

	return nil
}

// evalValidation runs the model over validation samples and returns average loss.
func evalValidation(internal mlx.InternalModel, adapter *mlx.LoRAAdapter, samples []trainSample, maxSeqLen int) float64 {
	maxBatches := 25
	if maxBatches > len(samples) {
		maxBatches = len(samples)
	}

	var totalLoss float64
	var count int

	for i := range maxBatches {
		sample := samples[i]
		seqLen := len(sample.Tokens)
		if seqLen < 2 {
			continue
		}

		inputTokens := sample.Tokens[:seqLen-1]
		targetTokens := sample.Tokens[1:]
		maskTokens := sample.Mask[1:]

		inputArr := mlx.FromValues(inputTokens, 1, len(inputTokens))
		targetArr := mlx.FromValues(targetTokens, 1, len(targetTokens))

		maskF32 := make([]float32, len(maskTokens))
		for j, m := range maskTokens {
			maskF32[j] = float32(m)
		}
		maskArr := mlx.FromValues(maskF32, 1, len(maskF32))
		mlx.Materialize(inputArr, targetArr, maskArr)

		caches := internal.NewCache()
		logits := internal.Forward(inputArr, caches)
		loss := mlx.MaskedCrossEntropyLoss(logits, targetArr, maskArr)
		mlx.Materialize(loss)

		totalLoss += loss.Float()
		count++
	}

	if count == 0 {
		return 0
	}
	return totalLoss / float64(count)
}

// queueLiveScore generates a response to a training prompt and queues it for scoring.
func queueLiveScore(ctx context.Context, tm inference.TrainableModel, samples []trainSample, iter int, influx *ml.InfluxClient, modelTag string) {
	sample := samples[iter%len(samples)]
	if len(sample.Tokens) < 2 {
		return
	}

	// Decode just the user prompt (tokens before the mask boundary)
	var promptEnd int
	for i, m := range sample.Mask {
		if m == 1 {
			promptEnd = i
			break
		}
	}
	if promptEnd == 0 {
		promptEnd = len(sample.Tokens) / 2
	}
	prompt := tm.Decode(sample.Tokens[:promptEnd])

	// Generate a live response
	var response strings.Builder
	for tok := range tm.Generate(ctx, prompt, inference.WithMaxTokens(128), inference.WithTemperature(0.7)) {
		response.WriteString(tok.Text)
	}

	// Queue to InfluxDB for scoring
	lp := fmt.Sprintf(
		`scoring_queue,model=%s,run_id=%s,phase=%s,status=pending iteration=%di,prompt="%s",response="%s"`,
		modelTag, ml.EscapeLp(trainRunID), ml.EscapeLp(trainPhase),
		iter, escapeFieldStr(prompt, 2000), escapeFieldStr(response.String(), 2000),
	)
	_ = influx.WriteLp([]string{lp})

	slog.Info("score job queued", "iter", iter, "response_len", response.Len())
}

// saveCheckpoint saves adapter weights and config at a training iteration.
func saveCheckpoint(adapter inference.Adapter, dir string, iter int, cfg inference.LoRAConfig) error {
	ckptFile := filepath.Join(dir, fmt.Sprintf("%07d_adapters.safetensors", iter))
	if err := adapter.Save(ckptFile); err != nil {
		return err
	}
	// Also save the latest
	if err := adapter.Save(filepath.Join(dir, "adapters.safetensors")); err != nil {
		return err
	}
	return writeAdapterConfig(dir, cfg)
}

// writeAdapterConfig writes adapter_config.json so mlx_lm can reload the adapter.
func writeAdapterConfig(dir string, cfg inference.LoRAConfig) error {
	config := map[string]any{
		"fine_tune_type": "lora",
		"rank":           cfg.Rank,
		"alpha":          cfg.Alpha,
		"lora_layers":    cfg.TargetKeys,
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "adapter_config.json"), data, 0o644)
}

func escapeFieldStr(s string, max int) string {
	s = truncate(s, max)
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}

// --- Data loading ---

func loadTrainingSamples(path string, tm inference.TrainableModel, maxSeqLen int) ([]trainSample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	modelType := tm.ModelType()
	var samples []trainSample
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

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

		sample := tokeniseConversation(entry.Messages, tm, modelType, maxSeqLen)
		if sample != nil {
			samples = append(samples, *sample)
		}
	}

	return samples, scanner.Err()
}

func tokeniseConversation(messages []ml.Message, tm inference.TrainableModel, modelType string, maxSeqLen int) *trainSample {
	fullText := formatConversation(messages, modelType, true)
	fullTokens := tm.Encode(fullText)

	if len(fullTokens) < 2 {
		return nil
	}
	if len(fullTokens) > maxSeqLen {
		fullTokens = fullTokens[:maxSeqLen]
	}

	prefixText := formatConversation(messages, modelType, false)
	prefixTokens := tm.Encode(prefixText)

	mask := make([]int32, len(fullTokens))
	for i := range mask {
		if i >= len(prefixTokens) {
			mask[i] = 1
		}
	}

	return &trainSample{Tokens: fullTokens, Mask: mask}
}

func formatConversation(messages []ml.Message, modelType string, includeAssistant bool) string {
	switch modelType {
	case "qwen3", "qwen2":
		return formatQwen3Train(messages, includeAssistant)
	default:
		return formatGemmaTrain(messages, includeAssistant)
	}
}

func formatQwen3Train(messages []ml.Message, includeAssistant bool) string {
	var sb strings.Builder
	for _, msg := range messages {
		if msg.Role == "assistant" && !includeAssistant {
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
