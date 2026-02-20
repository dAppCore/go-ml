package ml

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// RunAgentLoop is the main scoring agent loop.
func RunAgentLoop(cfg *AgentConfig) {
	log.Println(strings.Repeat("=", 60))
	log.Println("ROCm Scoring Agent — Go Edition")
	log.Printf("M3: %s@%s", cfg.M3User, cfg.M3Host)
	log.Printf("Inference API: %s", cfg.APIURL)
	log.Printf("Judge API: %s (%s)", cfg.JudgeURL, cfg.JudgeModel)
	log.Printf("InfluxDB: %s/%s", cfg.InfluxURL, cfg.InfluxDB)
	if cfg.DBPath != "" {
		log.Printf("DuckDB: %s", cfg.DBPath)
	}
	log.Printf("Poll interval: %ds", cfg.PollInterval)
	log.Println(strings.Repeat("=", 60))

	influx := NewInfluxClient(cfg.InfluxURL, cfg.InfluxDB)
	os.MkdirAll(cfg.WorkDir, 0755)

	for {
		ReplayInfluxBuffer(cfg.WorkDir, influx)

		log.Println("Discovering checkpoints on M3...")
		checkpoints, err := DiscoverCheckpoints(cfg)
		if err != nil {
			log.Printf("Discovery failed: %v", err)
			if cfg.OneShot {
				return
			}
			time.Sleep(time.Duration(cfg.PollInterval) * time.Second)
			continue
		}
		log.Printf("Found %d total checkpoints", len(checkpoints))

		var unscored []Checkpoint
		if cfg.Force {
			unscored = checkpoints
			log.Printf("Force mode: scoring all %d checkpoints", len(unscored))
		} else {
			scored, err := GetScoredLabels(influx)
			if err != nil {
				log.Printf("InfluxDB query failed: %v", err)
			}
			log.Printf("Already scored: %d (run_id, label) pairs", len(scored))
			unscored = FindUnscored(checkpoints, scored)
			log.Printf("Unscored: %d checkpoints", len(unscored))
		}

		if len(unscored) == 0 {
			log.Printf("Nothing to score. Sleeping %ds...", cfg.PollInterval)
			if cfg.OneShot {
				return
			}
			time.Sleep(time.Duration(cfg.PollInterval) * time.Second)
			continue
		}

		targets := unscored
		if !cfg.Force {
			targets = unscored[:1]
		}

		for i, target := range targets {
			log.Printf("Grabbed: %s (%s) [%d/%d]", target.Label, target.Dirname, i+1, len(targets))

			if cfg.DryRun {
				log.Printf("[DRY RUN] Would process: %s/%s", target.Dirname, target.Filename)
				continue
			}

			if err := ProcessOne(cfg, influx, target); err != nil {
				log.Printf("Error processing %s: %v", target.Label, err)
			}
			time.Sleep(5 * time.Second)
		}

		if cfg.DryRun || cfg.OneShot {
			return
		}
	}
}

// DiscoverCheckpoints lists all adapter directories and checkpoint files on M3 via SSH.
func DiscoverCheckpoints(cfg *AgentConfig) ([]Checkpoint, error) {
	pattern := "adapters-*"
	if cfg.Filter != "" {
		pattern = "adapters-" + cfg.Filter + "*"
	}
	out, err := SSHCommand(cfg, fmt.Sprintf("ls -d %s/%s 2>/dev/null", cfg.M3AdapterBase, pattern))
	if err != nil {
		return nil, fmt.Errorf("list adapter dirs: %w", err)
	}

	var checkpoints []Checkpoint
	iterRe := regexp.MustCompile(`(\d+)`)

	var adapterDirs []string
	for _, dirpath := range strings.Split(strings.TrimSpace(out), "\n") {
		if dirpath == "" {
			continue
		}
		subOut, subErr := SSHCommand(cfg, fmt.Sprintf("ls -d %s/gemma-3-* 2>/dev/null", dirpath))
		if subErr == nil && strings.TrimSpace(subOut) != "" {
			for _, sub := range strings.Split(strings.TrimSpace(subOut), "\n") {
				if sub != "" {
					adapterDirs = append(adapterDirs, sub)
				}
			}
		} else {
			adapterDirs = append(adapterDirs, dirpath)
		}
	}

	for _, dirpath := range adapterDirs {
		dirname := strings.TrimPrefix(dirpath, cfg.M3AdapterBase+"/")

		filesOut, err := SSHCommand(cfg, fmt.Sprintf("ls %s/*_adapters.safetensors 2>/dev/null", dirpath))
		if err != nil {
			continue
		}

		for _, fp := range strings.Split(strings.TrimSpace(filesOut), "\n") {
			if fp == "" {
				continue
			}
			filename := fileBase(fp)

			match := iterRe.FindStringSubmatch(filename)
			if len(match) < 2 {
				continue
			}
			iteration := 0
			fmt.Sscanf(match[1], "%d", &iteration)

			modelTag, labelPrefix, stem := AdapterMeta(dirname)
			label := fmt.Sprintf("%s @%s", labelPrefix, match[1])
			runID := fmt.Sprintf("%s-capability-auto", stem)

			checkpoints = append(checkpoints, Checkpoint{
				RemoteDir: dirpath,
				Filename:  filename,
				Dirname:   dirname,
				Iteration: iteration,
				ModelTag:  modelTag,
				Label:     label,
				RunID:     runID,
			})
		}
	}

	return checkpoints, nil
}

// GetScoredLabels returns all (run_id, label) pairs already scored in InfluxDB.
func GetScoredLabels(influx *InfluxClient) (map[[2]string]bool, error) {
	rows, err := influx.QuerySQL("SELECT DISTINCT run_id, label FROM capability_score")
	if err != nil {
		return nil, err
	}

	scored := make(map[[2]string]bool)
	for _, row := range rows {
		runID, _ := row["run_id"].(string)
		label, _ := row["label"].(string)
		if runID != "" && label != "" {
			scored[[2]string{runID, label}] = true
		}
	}
	return scored, nil
}

// FindUnscored filters checkpoints to only unscored ones, sorted by (dirname, iteration).
func FindUnscored(checkpoints []Checkpoint, scored map[[2]string]bool) []Checkpoint {
	var unscored []Checkpoint
	for _, c := range checkpoints {
		if !scored[[2]string{c.RunID, c.Label}] {
			unscored = append(unscored, c)
		}
	}
	sort.Slice(unscored, func(i, j int) bool {
		if unscored[i].Dirname != unscored[j].Dirname {
			return unscored[i].Dirname < unscored[j].Dirname
		}
		return unscored[i].Iteration < unscored[j].Iteration
	})
	return unscored
}

// isMLXNative returns true if this model can be served directly on M3 via
// mlx_lm.server with --adapter, avoiding the MLX→PEFT conversion step.
func isMLXNative(modelTag string) bool {
	return strings.HasPrefix(modelTag, "gemma-3-") || strings.HasPrefix(modelTag, "gpt-oss")
}

// ProcessOne fetches, converts, scores, and pushes one checkpoint.
func ProcessOne(cfg *AgentConfig, influx *InfluxClient, cp Checkpoint) error {
	log.Println(strings.Repeat("=", 60))
	log.Printf("Processing: %s / %s [%s]", cp.Dirname, cp.Filename, cp.ModelTag)
	log.Println(strings.Repeat("=", 60))

	if isMLXNative(cp.ModelTag) {
		return processMLXNative(cfg, influx, cp)
	}
	return processWithConversion(cfg, influx, cp)
}
