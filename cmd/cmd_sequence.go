//go:build darwin && arm64 && !nomlx && cliv1

package cmd

import (
	"io"
	"log/slog"
	"time"

	"dappco.re/go/core"
	"dappco.re/go/cli/pkg/cli"
	coreio "dappco.re/go/io"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
	"gopkg.in/yaml.v3"
)

var sequenceCmd = &cli.Command{
	Use:   "sequence",
	Short: "Run a training sequence of multiple lessons",
	Long: `Runs an ordered sequence of lessons defined in a YAML file.

Sequence YAML format:
  id: lek-full
  title: "LEK Full Training Sequence"
  mode: vertical
  model-path: /path/to/model
  lessons:
    - sovereignty.yaml
    - privacy.yaml
    - censorship.yaml

Mode:
  vertical   Run lessons strictly in order (default)
  horizontal Run all lessons, order doesn't matter

State is tracked per-sequence so runs can be resumed.`,
	RunE: runSequence,
}

var (
	sequenceFile      string
	sequenceModelPath string
	sequenceOutput    string
	sequenceMaxTokens int
	sequenceTemp      float64
	sequenceMemLimit  int
)

func init() {
	sequenceCmd.Flags().StringVar(&sequenceFile, "file", "", "Sequence YAML file (required)")
	sequenceCmd.Flags().StringVar(&sequenceModelPath, "model-path", "", "Path to model directory (required)")
	sequenceCmd.Flags().StringVar(&sequenceOutput, "output", "", "Output JSONL file (default: <sequence-id>.jsonl)")
	sequenceCmd.Flags().IntVar(&sequenceMaxTokens, "max-tokens", 1024, "Max tokens per response")
	sequenceCmd.Flags().Float64Var(&sequenceTemp, "temperature", 0.4, "Sampling temperature")
	sequenceCmd.Flags().IntVar(&sequenceMemLimit, "memory-limit", 24, "Metal memory limit in GB")
	sequenceCmd.MarkFlagRequired("file")
	sequenceCmd.MarkFlagRequired("model-path")
}

// sequenceDef is a YAML sequence definition.
type sequenceDef struct {
	ID        string   `yaml:"id"`
	Title     string   `yaml:"title"`
	Mode      string   `yaml:"mode"` // "vertical" (default) or "horizontal"
	ModelPath string   `yaml:"model-path"`
	Lessons   []string `yaml:"lessons"` // Relative paths to lesson YAML files
}

// sequenceState tracks progress through a sequence.
type sequenceState struct {
	SequenceID string          `json:"sequence_id"`
	Completed  map[string]bool `json:"completed"` // lesson ID → done
	Current    string          `json:"current"`
	UpdatedAt  string          `json:"updated_at"`
}

func runSequence(cmd *cli.Command, args []string) error {
	start := time.Now()

	// Load sequence YAML
	data, err := coreio.Local.Read(sequenceFile)
	if err != nil {
		return coreerr.E("cmd.runSequence", "read sequence", err)
	}

	var seq sequenceDef
	if err := yaml.Unmarshal([]byte(data), &seq); err != nil {
		return coreerr.E("cmd.runSequence", "parse sequence", err)
	}

	if seq.ID == "" {
		seq.ID = core.TrimSuffix(core.PathBase(sequenceFile), core.PathExt(sequenceFile))
	}
	if seq.Mode == "" {
		seq.Mode = "vertical"
	}

	// Model path from sequence or flag
	modelPath := sequenceModelPath
	if modelPath == "" && seq.ModelPath != "" {
		modelPath = seq.ModelPath
	}
	if modelPath == "" {
		return coreerr.E("cmd.runSequence", "model-path is required (flag or sequence YAML)", nil)
	}

	// Resolve output
	if sequenceOutput == "" {
		sequenceOutput = seq.ID + ".jsonl"
	}

	slog.Info("sequence: loaded",
		"id", seq.ID,
		"title", seq.Title,
		"mode", seq.Mode,
		"lessons", len(seq.Lessons),
	)

	// Load state
	stateFile := seq.ID + ".sequence-state.json"
	state := loadSequenceState(stateFile)
	if state.SequenceID == "" {
		state.SequenceID = seq.ID
		state.Completed = make(map[string]bool)
	}

	// Load model once for all lessons
	slog.Info("sequence: loading model", "path", modelPath)
	backend, err := ml.NewMLXBackend(modelPath)
	if err != nil {
		return coreerr.E("cmd.runSequence", "load model", err)
	}

	opts := ml.GenOpts{
		Temperature: sequenceTemp,
		MaxTokens:   sequenceMaxTokens,
	}

	// Open output file
	outFile, err := coreio.Local.Append(sequenceOutput)
	if err != nil {
		return coreerr.E("cmd.runSequence", "create output", err)
	}
	defer outFile.Close()

	baseDir := core.PathDir(sequenceFile)
	totalGenerated := 0

	for i, lessonPath := range seq.Lessons {
		// Resolve lesson path
		if !core.PathIsAbs(lessonPath) {
			lessonPath = core.JoinPath(baseDir, lessonPath)
		}

		// Load lesson
		lessonData, err := coreio.Local.Read(lessonPath)
		if err != nil {
			slog.Error("sequence: failed to read lesson",
				"path", lessonPath,
				"error", err,
			)
			if seq.Mode == "vertical" {
				return coreerr.E("cmd.runSequence", "vertical sequence halted", err)
			}
			continue
		}

		var lesson lessonDef
		if err := yaml.Unmarshal([]byte(lessonData), &lesson); err != nil {
			slog.Error("sequence: failed to parse lesson",
				"path", lessonPath,
				"error", err,
			)
			if seq.Mode == "vertical" {
				return coreerr.E("cmd.runSequence", "vertical sequence halted", err)
			}
			continue
		}

		if lesson.ID == "" {
			lesson.ID = core.TrimSuffix(core.PathBase(lessonPath), core.PathExt(lessonPath))
		}

		// Skip completed lessons
		if state.Completed[lesson.ID] {
			slog.Info("sequence: skipping completed lesson",
				"lesson", core.Sprintf("%d/%d", i+1, len(seq.Lessons)),
				"id", lesson.ID,
			)
			continue
		}

		state.Current = lesson.ID

		slog.Info("sequence: starting lesson",
			"lesson", core.Sprintf("%d/%d", i+1, len(seq.Lessons)),
			"id", lesson.ID,
			"title", lesson.Title,
			"prompts", len(lesson.Prompts),
		)

		// Load sandwich files for this lesson
		var kbText, kernelText string
		hasSandwich := false
		if lesson.Sandwich != nil {
			lessonDir := core.PathDir(lessonPath)
			if lesson.Sandwich.KB != "" {
				kbPath := lesson.Sandwich.KB
				if !core.PathIsAbs(kbPath) {
					kbPath = core.JoinPath(lessonDir, kbPath)
				}
				d, err := coreio.Local.Read(kbPath)
				if err != nil {
					slog.Error("sequence: failed to read KB", "error", err)
				} else {
					kbText = d
				}
			}
			if lesson.Sandwich.Kernel != "" {
				kernelPath := lesson.Sandwich.Kernel
				if !core.PathIsAbs(kernelPath) {
					kernelPath = core.JoinPath(lessonDir, kernelPath)
				}
				d, err := coreio.Local.Read(kernelPath)
				if err != nil {
					slog.Error("sequence: failed to read kernel", "error", err)
				} else {
					kernelText = d
				}
			}
			hasSandwich = kbText != "" && kernelText != ""
		}

		// Run each prompt in the lesson
		generated := 0
		for j, prompt := range lesson.Prompts {
			var messages []ml.Message
			if lesson.System != "" {
				messages = append(messages, ml.Message{Role: "system", Content: lesson.System})
			}

			userContent := prompt.Prompt
			if hasSandwich {
				userContent = buildSandwich(kbText, prompt.Prompt, kernelText)
			}
			messages = append(messages, ml.Message{Role: "user", Content: userContent})

			slog.Info("sequence: generating",
				"lesson", lesson.ID,
				"prompt", core.Sprintf("%d/%d", j+1, len(lesson.Prompts)),
				"id", prompt.ID,
			)

			res, err := backend.Chat(cmd.Context(), messages, opts)
			if err != nil {
				slog.Error("sequence: generation failed",
					"lesson", lesson.ID,
					"prompt", prompt.ID,
					"error", err,
				)
				continue
			}

			response := res.Text
			record := struct {
				Messages []ml.Message `json:"messages"`
			}{
				Messages: []ml.Message{
					{Role: "user", Content: userContent},
					{Role: "assistant", Content: response},
				},
			}
			if _, err := io.WriteString(outFile, core.Concat(core.JSONMarshalString(record), "\n")); err != nil {
				return coreerr.E("cmd.runSequence", "write record", err)
			}

			generated++
			totalGenerated++
		}

		// Mark lesson complete
		state.Completed[lesson.ID] = true
		state.UpdatedAt = time.Now().Format(time.RFC3339)
		saveSequenceState(stateFile, state)

		slog.Info("sequence: lesson complete",
			"id", lesson.ID,
			"generated", generated,
			"total", len(lesson.Prompts),
		)
	}

	state.Current = ""
	state.UpdatedAt = time.Now().Format(time.RFC3339)
	saveSequenceState(stateFile, state)

	slog.Info("sequence: complete",
		"id", seq.ID,
		"output", sequenceOutput,
		"total_generated", totalGenerated,
		"lessons_completed", len(state.Completed),
		"duration", time.Since(start).Round(time.Second),
	)

	return nil
}

func loadSequenceState(path string) sequenceState {
	data, err := coreio.Local.Read(path)
	if err != nil {
		return sequenceState{}
	}
	var state sequenceState
	if r := core.JSONUnmarshalString(data, &state); !r.OK {
		return sequenceState{}
	}
	return state
}

func saveSequenceState(path string, state sequenceState) {
	_ = coreio.Local.Write(path, core.JSONMarshalString(state))
}
