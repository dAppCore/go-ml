package ml

import (
	"context"
	"iter"
	"regexp"
	"slices"
<<<<<<< HEAD
=======
	"strconv"
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	"time"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
)

// RunAgentLoop is the main scoring agent loop.
func RunAgentLoop(cfg *AgentConfig) {
<<<<<<< HEAD
	core.Print(nil, repeatStr("=", LogSeparatorWidth))
	core.Print(nil, "ROCm Scoring Agent — Go Edition")
	core.Print(nil, "M3: %s@%s", cfg.M3User, cfg.M3Host)
	core.Print(nil, "Inference API: %s", cfg.APIURL)
	core.Print(nil, "Judge API: %s (%s)", cfg.JudgeURL, cfg.JudgeModel)
	core.Print(nil, "InfluxDB: %s/%s", cfg.InfluxURL, cfg.InfluxDB)
=======
	log.Println(repeatString("=", LogSeparatorWidth))
	log.Println("ROCm Scoring Agent — Go Edition")
	log.Printf("M3: %s@%s", cfg.M3User, cfg.M3Host)
	log.Printf("Inference API: %s", cfg.APIURL)
	log.Printf("Judge API: %s (%s)", cfg.JudgeURL, cfg.JudgeModel)
	log.Printf("InfluxDB: %s/%s", cfg.InfluxURL, cfg.InfluxDB)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	if cfg.DBPath != "" {
		core.Print(nil, "DuckDB: %s", cfg.DBPath)
	}
<<<<<<< HEAD
	core.Print(nil, "Poll interval: %ds", cfg.PollInterval)
	core.Print(nil, repeatStr("=", LogSeparatorWidth))
=======
	log.Printf("Poll interval: %ds", cfg.PollInterval)
	log.Println(repeatString("=", LogSeparatorWidth))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

	influx := NewInfluxClient(cfg.InfluxURL, cfg.InfluxDB)
	coreio.Local.EnsureDir(cfg.WorkDir)

	for {
		ReplayInfluxBuffer(cfg.WorkDir, influx)

		core.Print(nil, "Discovering checkpoints on M3...")
		checkpoints, err := DiscoverCheckpoints(cfg)
		if err != nil {
			core.Print(nil, "Discovery failed: %v", err)
			if cfg.OneShot {
				return
			}
			time.Sleep(time.Duration(cfg.PollInterval) * time.Second)
			continue
		}
		core.Print(nil, "Found %d total checkpoints", len(checkpoints))

		var unscored []Checkpoint
		if cfg.Force {
			unscored = checkpoints
			core.Print(nil, "Force mode: scoring all %d checkpoints", len(unscored))
		} else {
			scored, err := GetScoredLabels(influx)
			if err != nil {
				core.Print(nil, "InfluxDB query failed: %v", err)
			}
			core.Print(nil, "Already scored: %d (run_id, label) pairs", len(scored))
			unscored = FindUnscored(checkpoints, scored)
			core.Print(nil, "Unscored: %d checkpoints", len(unscored))
		}

		if len(unscored) == 0 {
			core.Print(nil, "Nothing to score. Sleeping %ds...", cfg.PollInterval)
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
			core.Print(nil, "Grabbed: %s (%s) [%d/%d]", target.Label, target.Dirname, i+1, len(targets))

			if cfg.DryRun {
				core.Print(nil, "[DRY RUN] Would process: %s/%s", target.Dirname, target.Filename)
				continue
			}

			if err := ProcessOne(cfg, influx, target); err != nil {
				core.Print(nil, "Error processing %s: %v", target.Label, err)
			}
			time.Sleep(InterCheckpointDelay)
		}

		if cfg.DryRun || cfg.OneShot {
			return
		}
	}
}

// DiscoverCheckpoints lists all adapter directories and checkpoint files on M3 via SSH.
func DiscoverCheckpoints(cfg *AgentConfig) ([]Checkpoint, error) {
	var checkpoints []Checkpoint
	for cp, err := range DiscoverCheckpointsIter(cfg) {
		if err != nil {
			return nil, err
		}
		checkpoints = append(checkpoints, cp)
	}
	return checkpoints, nil
}

// DiscoverCheckpointsIter returns an iterator over discovered adapter checkpoints.
func DiscoverCheckpointsIter(cfg *AgentConfig) iter.Seq2[Checkpoint, error] {
	return func(yield func(Checkpoint, error) bool) {
		pattern := "adapters-*"
		if cfg.Filter != "" {
			pattern = "adapters-" + cfg.Filter + "*"
		}
		t := cfg.transport()
		ctx := context.Background()
		out, err := t.Run(ctx, core.Sprintf("ls -d %s/%s 2>/dev/null", cfg.M3AdapterBase, pattern))
		if err != nil {
			yield(Checkpoint{}, coreerr.E("ml.DiscoverCheckpointsIter", "list adapter dirs", err))
			return
		}

		iterRe := regexp.MustCompile(`(\d+)`)

		var adapterDirs []string
<<<<<<< HEAD
		for dirpath := range splitSeq(core.Trim(out), "\n") {
=======
		for _, dirpath := range core.Split(core.Trim(out), "\n") {
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
			if dirpath == "" {
				continue
			}
			subOut, subErr := t.Run(ctx, core.Sprintf("ls -d %s/gemma-3-* 2>/dev/null", dirpath))
			if subErr == nil && core.Trim(subOut) != "" {
<<<<<<< HEAD
				for sub := range splitSeq(core.Trim(subOut), "\n") {
=======
				for _, sub := range core.Split(core.Trim(subOut), "\n") {
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
					if sub != "" {
						adapterDirs = append(adapterDirs, sub)
					}
				}
			} else {
				adapterDirs = append(adapterDirs, dirpath)
			}
		}

		for _, dirpath := range adapterDirs {
<<<<<<< HEAD
			dirname := core.TrimPrefix(dirpath, cfg.M3AdapterBase+"/")
=======
			dirname := core.TrimPrefix(dirpath, core.Concat(cfg.M3AdapterBase, "/"))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

			filesOut, err := t.Run(ctx, core.Sprintf("ls %s/*_adapters.safetensors 2>/dev/null", dirpath))
			if err != nil {
				continue
			}

<<<<<<< HEAD
			for fp := range splitSeq(core.Trim(filesOut), "\n") {
=======
			for _, fp := range core.Split(core.Trim(filesOut), "\n") {
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
				if fp == "" {
					continue
				}
				filename := fileBase(fp)

				match := iterRe.FindStringSubmatch(filename)
				if len(match) < 2 {
					continue
				}
<<<<<<< HEAD
				iteration := 0
				sscanf(match[1], "%d", &iteration)
=======
				iteration, _ := strconv.Atoi(match[1])
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

				modelTag, labelPrefix, stem := AdapterMeta(dirname)
				label := core.Sprintf("%s @%s", labelPrefix, match[1])
				runID := core.Sprintf("%s-capability-auto", stem)

				if !yield(Checkpoint{
					RemoteDir: dirpath,
					Filename:  filename,
					Dirname:   dirname,
					Iteration: iteration,
					ModelTag:  modelTag,
					Label:     label,
					RunID:     runID,
				}, nil) {
					return
				}
			}
		}
	}
}

// GetScoredLabels returns all (run_id, label) pairs already scored in InfluxDB.
func GetScoredLabels(influx *InfluxClient) (map[[2]string]bool, error) {
	rows, err := influx.QuerySQL("SELECT DISTINCT run_id, label FROM " + MeasurementCapabilityScore)
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
	for c := range FindUnscoredIter(checkpoints, scored) {
		unscored = append(unscored, c)
	}
	slices.SortFunc(unscored, func(a, b Checkpoint) int {
		if a.Dirname != b.Dirname {
<<<<<<< HEAD
			return compareStr(a.Dirname, b.Dirname)
=======
			if a.Dirname < b.Dirname {
				return -1
			}
			return 1
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
		}
		return a.Iteration - b.Iteration
	})
	return unscored
}

// FindUnscoredIter returns an iterator over checkpoints that have not yet been scored.
func FindUnscoredIter(checkpoints []Checkpoint, scored map[[2]string]bool) iter.Seq[Checkpoint] {
	return func(yield func(Checkpoint) bool) {
		for _, c := range checkpoints {
			if !scored[[2]string{c.RunID, c.Label}] {
				if !yield(c) {
					return
				}
			}
		}
	}
}

// isMLXNative returns true if this model can be served directly on M3 via
// mlx_lm.server with --adapter, avoiding the MLX→PEFT conversion step.
func isMLXNative(modelTag string) bool {
	return core.HasPrefix(modelTag, "gemma-3-") || core.HasPrefix(modelTag, "gpt-oss")
}

// ProcessOne fetches, converts, scores, and pushes one checkpoint.
func ProcessOne(cfg *AgentConfig, influx *InfluxClient, cp Checkpoint) error {
<<<<<<< HEAD
	core.Print(nil, repeatStr("=", LogSeparatorWidth))
	core.Print(nil, "Processing: %s / %s [%s]", cp.Dirname, cp.Filename, cp.ModelTag)
	core.Print(nil, repeatStr("=", LogSeparatorWidth))
=======
	log.Println(repeatString("=", LogSeparatorWidth))
	log.Printf("Processing: %s / %s [%s]", cp.Dirname, cp.Filename, cp.ModelTag)
	log.Println(repeatString("=", LogSeparatorWidth))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

	if isMLXNative(cp.ModelTag) {
		return processMLXNative(cfg, influx, cp)
	}
	return processWithConversion(cfg, influx, cp)
}
