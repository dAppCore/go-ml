//go:build darwin && arm64 && !nomlx

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"forge.lthn.ai/core/cli/pkg/cli"

	"gopkg.in/yaml.v3"
)

var lessonCmd = &cli.Command{
	Use:   "lesson",
	Short: "Run a structured training lesson from a YAML definition",
	Long: `Runs a training lesson defined in a YAML file. Each lesson contains
prompts organised by category, optional system prompt, and sandwich
signing configuration.

Lesson YAML format:
  id: lek-sovereignty
  title: "Sovereignty Lessons"
  system: "You are a helpful assistant."
  sandwich:
    kb: path/to/axioms.md
    kernel: path/to/kernel.txt
  prompts:
    - id: P01
      category: sovereignty
      prompt: "A user wants to build an auth system."
      signal: "Does the model prefer decentralised?"

The command generates responses for each prompt and writes them as
training JSONL. State is tracked so lessons can be resumed.`,
	RunE: runLesson,
}

var (
	lessonFile      string
	lessonModelPath string
	lessonOutput    string
	lessonMaxTokens int
	lessonTemp      float64
	lessonMemLimit  int
	lessonResume    bool
	lessonInteract  bool
)

func init() {
	lessonCmd.Flags().StringVar(&lessonFile, "file", "", "Lesson YAML file (required)")
	lessonCmd.Flags().StringVar(&lessonModelPath, "model-path", "", "Path to model directory (required)")
	lessonCmd.Flags().StringVar(&lessonOutput, "output", "", "Output JSONL file (default: <lesson-id>.jsonl)")
	lessonCmd.Flags().IntVar(&lessonMaxTokens, "max-tokens", 1024, "Max tokens per response")
	lessonCmd.Flags().Float64Var(&lessonTemp, "temperature", 0.4, "Sampling temperature")
	lessonCmd.Flags().IntVar(&lessonMemLimit, "memory-limit", 24, "Metal memory limit in GB")
	lessonCmd.Flags().BoolVar(&lessonResume, "resume", true, "Resume from last completed prompt")
	lessonCmd.Flags().BoolVar(&lessonInteract, "interactive", false, "Interactive mode: review each response before continuing")
	lessonCmd.MarkFlagRequired("file")
	lessonCmd.MarkFlagRequired("model-path")
}

// lessonDef is a YAML lesson definition.
type lessonDef struct {
	ID       string             `yaml:"id"`
	Title    string             `yaml:"title"`
	System   string             `yaml:"system"`
	Sandwich *lessonSandwichCfg `yaml:"sandwich"`
	Prompts  []lessonPrompt     `yaml:"prompts"`
}

type lessonSandwichCfg struct {
	KB     string `yaml:"kb"`
	Kernel string `yaml:"kernel"`
}

type lessonPrompt struct {
	ID       string `yaml:"id"`
	Category string `yaml:"category"`
	Prompt   string `yaml:"prompt"`
	Signal   string `yaml:"signal"`
}

// lessonState tracks progress through a lesson.
type lessonState struct {
	LessonID  string                  `json:"lesson_id"`
	Completed map[string]lessonResult `json:"completed"`
	UpdatedAt string                  `json:"updated_at"`
}

type lessonResult struct {
	ResponseChars int    `json:"response_chars"`
	Duration      string `json:"duration"`
	CompletedAt   string `json:"completed_at"`
}

func runLesson(cmd *cli.Command, args []string) error {
	start := time.Now()

	// Load lesson YAML
	data, err := coreio.Local.Read(lessonFile)
	if err != nil {
		return coreerr.E("cmd.runLesson", "read lesson", err)
	}

	var lesson lessonDef
	if err := yaml.Unmarshal([]byte(data), &lesson); err != nil {
		return coreerr.E("cmd.runLesson", "parse lesson", err)
	}

	if lesson.ID == "" {
		lesson.ID = strings.TrimSuffix(filepath.Base(lessonFile), filepath.Ext(lessonFile))
	}

	// Resolve output path
	if lessonOutput == "" {
		lessonOutput = lesson.ID + ".jsonl"
	}

	// Load sandwich files if configured
	var kbText, kernelText string
	sandwich := false
	if lesson.Sandwich != nil {
		baseDir := filepath.Dir(lessonFile)
		if lesson.Sandwich.KB != "" {
			kbPath := lesson.Sandwich.KB
			if !filepath.IsAbs(kbPath) {
				kbPath = filepath.Join(baseDir, kbPath)
			}
			d, err := coreio.Local.Read(kbPath)
			if err != nil {
				return coreerr.E("cmd.runLesson", "read KB", err)
			}
			kbText = d
		}
		if lesson.Sandwich.Kernel != "" {
			kernelPath := lesson.Sandwich.Kernel
			if !filepath.IsAbs(kernelPath) {
				kernelPath = filepath.Join(baseDir, kernelPath)
			}
			d, err := coreio.Local.Read(kernelPath)
			if err != nil {
				return coreerr.E("cmd.runLesson", "read kernel", err)
			}
			kernelText = d
		}
		sandwich = kbText != "" && kernelText != ""
	}

	slog.Info("lesson: loaded",
		"id", lesson.ID,
		"title", lesson.Title,
		"prompts", len(lesson.Prompts),
		"sandwich", sandwich,
	)

	if len(lesson.Prompts) == 0 {
		return coreerr.E("cmd.runLesson", "lesson has no prompts", nil)
	}

	// Load state for resume
	stateFile := lesson.ID + ".state.json"
	state := loadLessonState(stateFile)
	if state.LessonID == "" {
		state.LessonID = lesson.ID
		state.Completed = make(map[string]lessonResult)
	}

	// Count remaining
	var remaining []lessonPrompt
	for _, p := range lesson.Prompts {
		if lessonResume {
			if _, done := state.Completed[p.ID]; done {
				continue
			}
		}
		remaining = append(remaining, p)
	}

	if len(remaining) == 0 {
		slog.Info("lesson: all prompts completed",
			"id", lesson.ID,
			"total", len(lesson.Prompts),
		)
		return nil
	}

	slog.Info("lesson: starting",
		"remaining", len(remaining),
		"completed", len(state.Completed),
		"total", len(lesson.Prompts),
	)

	// Load model
	slog.Info("lesson: loading model", "path", lessonModelPath)
	backend, err := ml.NewMLXBackend(lessonModelPath)
	if err != nil {
		return coreerr.E("cmd.runLesson", "load model", err)
	}

	opts := ml.GenOpts{
		Temperature: lessonTemp,
		MaxTokens:   lessonMaxTokens,
	}

	// Open output file (append mode for resume)
	outFile, err := coreio.Local.Append(lessonOutput)
	if err != nil {
		return coreerr.E("cmd.runLesson", "create output", err)
	}
	defer outFile.Close()
	encoder := json.NewEncoder(outFile)

	generated := 0

	for i, prompt := range remaining {
		promptStart := time.Now()

		slog.Info("lesson: generating",
			"prompt", fmt.Sprintf("%d/%d", i+1, len(remaining)),
			"id", prompt.ID,
			"category", prompt.Category,
		)

		// Build messages
		var messages []ml.Message
		if lesson.System != "" {
			messages = append(messages, ml.Message{Role: "system", Content: lesson.System})
		}

		userContent := prompt.Prompt
		if sandwich {
			userContent = buildSandwich(kbText, prompt.Prompt, kernelText)
		}
		messages = append(messages, ml.Message{Role: "user", Content: userContent})

		// Generate
		res, err := backend.Chat(context.Background(), messages, opts)
		if err != nil {
			slog.Error("lesson: generation failed",
				"id", prompt.ID,
				"error", err,
			)
			continue
		}

		response := res.Text
		elapsed := time.Since(promptStart)

		// Write training record
		record := struct {
			Messages []ml.Message `json:"messages"`
		}{
			Messages: []ml.Message{
				{Role: "user", Content: userContent},
				{Role: "assistant", Content: response},
			},
		}
		if err := encoder.Encode(record); err != nil {
			return coreerr.E("cmd.runLesson", "write record", err)
		}

		// Update state
		state.Completed[prompt.ID] = lessonResult{
			ResponseChars: len(response),
			Duration:      elapsed.Round(time.Second).String(),
			CompletedAt:   time.Now().Format(time.RFC3339),
		}
		state.UpdatedAt = time.Now().Format(time.RFC3339)

		if err := saveLessonState(stateFile, state); err != nil {
			slog.Warn("lesson: failed to save state", "error", err)
		}

		generated++

		slog.Info("lesson: generated",
			"id", prompt.ID,
			"category", prompt.Category,
			"response_chars", len(response),
			"duration", elapsed.Round(time.Second),
		)

		// Interactive mode: show response and wait for confirmation
		if lessonInteract {
			fmt.Printf("\n--- %s (%s) ---\n", prompt.ID, prompt.Category)
			fmt.Printf("Prompt: %s\n\n", prompt.Prompt)
			if prompt.Signal != "" {
				fmt.Printf("Signal: %s\n\n", prompt.Signal)
			}
			fmt.Printf("Response:\n%s\n", response)
			fmt.Printf("\nPress Enter to continue (or 'q' to stop)... ")
			var input string
			fmt.Scanln(&input)
			if strings.TrimSpace(input) == "q" {
				break
			}
		}

		// Periodic cleanup
		if (i+1)%4 == 0 {
			runtime.GC()
		}
	}

	slog.Info("lesson: complete",
		"id", lesson.ID,
		"output", lessonOutput,
		"generated", generated,
		"total_completed", len(state.Completed),
		"total_prompts", len(lesson.Prompts),
		"duration", time.Since(start).Round(time.Second),
	)

	return nil
}

func loadLessonState(path string) lessonState {
	data, err := coreio.Local.Read(path)
	if err != nil {
		return lessonState{}
	}
	var state lessonState
	json.Unmarshal([]byte(data), &state)
	return state
}

func saveLessonState(path string, state lessonState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return coreio.Local.Write(path, string(data))
}
