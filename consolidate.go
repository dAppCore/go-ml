package ml

import (
	"dappco.re/go/core"
	"bufio"
<<<<<<< HEAD
	"encoding/json"
	"io"
	"maps"
	"os/exec"
=======
	"context"
	"io"
	"maps"
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	"slices"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
	goexec "dappco.re/go/core/process/exec"
)

// ConsolidateConfig holds options for the consolidate operation.
type ConsolidateConfig struct {
	M3Host    string
	RemoteDir string
	Pattern   string
	OutputDir string
	MergedOut string
}

// Consolidate pulls JSONL response files from M3 via SSH, merges them by idx,
// deduplicates, and writes a single merged JSONL output.
func Consolidate(cfg ConsolidateConfig, w io.Writer) error {
	if cfg.OutputDir == "" {
		cfg.OutputDir = "responses"
	}
	if err := coreio.Local.EnsureDir(cfg.OutputDir); err != nil {
		return coreerr.E("ml.Consolidate", "create output dir", err)
	}

	// List remote files via SSH.
<<<<<<< HEAD
	fprintf(w, "%s\n", "Pulling responses from remote...")
	listCmd := exec.Command("ssh", cfg.M3Host, core.Sprintf("ls %s/%s", cfg.RemoteDir, cfg.Pattern))
=======
	core.Print(w, "Pulling responses from remote...")
	listCmd := goexec.Command(context.Background(), "ssh", cfg.M3Host, core.Sprintf("ls %s/%s", cfg.RemoteDir, cfg.Pattern))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	listOutput, err := listCmd.Output()
	if err != nil {
		return coreerr.E("ml.Consolidate", "list remote files", err)
	}

	remoteFiles := core.Split(core.Trim(string(listOutput)), "\n")
	var validFiles []string
	for _, f := range remoteFiles {
		f = core.Trim(f)
		if f != "" {
			validFiles = append(validFiles, f)
		}
	}
<<<<<<< HEAD
	fprintf(w, "  Found %d JSONL files on %s\n", len(validFiles), cfg.M3Host)

	// Pull each file via SCP.
	for _, rf := range validFiles {
		local := core.Path(cfg.OutputDir, core.PathBase(rf))
		scpCmd := exec.Command("scp", core.Sprintf("%s:%s", cfg.M3Host, rf), local)
		if err := scpCmd.Run(); err != nil {
			fprintf(w, "  warning: failed to pull %s: %v\n", rf, err)
=======
	core.Print(w, "  Found %d JSONL files on %s", len(validFiles), cfg.M3Host)

	// Pull each file via SCP.
	for _, rf := range validFiles {
		local := core.JoinPath(cfg.OutputDir, core.PathBase(rf))
		scpCmd := goexec.Command(context.Background(), "scp", core.Sprintf("%s:%s", cfg.M3Host, rf), local)
		if err := scpCmd.Run(); err != nil {
			core.Print(w, "  warning: failed to pull %s: %v", rf, err)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
			continue
		}

		lines, err := countLines(local)
		if err == nil {
<<<<<<< HEAD
			fprintf(w, "  %s: %d records\n", core.PathBase(rf), lines)
=======
			core.Print(w, "  %s: %d records", core.PathBase(rf), lines)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
		}
	}

	// Merge and deduplicate on idx (first occurrence wins).
	seen := make(map[int]string)
	skipped := 0

<<<<<<< HEAD
	matches := core.PathGlob(core.Path(cfg.OutputDir, cfg.Pattern))
=======
	matches := core.PathGlob(core.JoinPath(cfg.OutputDir, cfg.Pattern))
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	slices.Sort(matches)

	for _, local := range matches {
		f, err := coreio.Local.Open(local)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			var rec struct {
				Idx *int `json:"idx"`
			}
			if r := core.JSONUnmarshalString(line, &rec); !r.OK {
				skipped++
				continue
			}
			if rec.Idx == nil {
				skipped++
				continue
			}
			if _, exists := seen[*rec.Idx]; !exists {
				seen[*rec.Idx] = line
			}
		}
		f.Close()
	}

	if skipped > 0 {
<<<<<<< HEAD
		fprintf(w, "  Skipped %d records without idx\n", skipped)
=======
		core.Print(w, "  Skipped %d records without idx", skipped)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	}

	// Sort by idx and write merged file.
	mergedPath := cfg.MergedOut
	if mergedPath == "" {
<<<<<<< HEAD
		mergedPath = core.Path(cfg.OutputDir, "..", "gold-merged.jsonl")
=======
		mergedPath = core.JoinPath(cfg.OutputDir, "..", "gold-merged.jsonl")
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	}

	idxs := slices.Sorted(maps.Keys(seen))

	out, err := coreio.Local.Create(mergedPath)
	if err != nil {
		return coreerr.E("ml.Consolidate", "create merged file", err)
	}
	defer out.Close()

	bw := bufio.NewWriter(out)
	for _, idx := range idxs {
		bw.WriteString(seen[idx])
		bw.WriteString("\n")
	}
	if err := bw.Flush(); err != nil {
		return coreerr.E("ml.Consolidate", "flush merged file", err)
	}

<<<<<<< HEAD
	fprintf(w, "\nMerged: %d unique examples -> %s\n", len(seen), mergedPath)
=======
	core.Print(w, "")
	core.Print(w, "Merged: %d unique examples -> %s", len(seen), mergedPath)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	return nil
}

// countLines returns the number of lines in a file.
func countLines(path string) (int, error) {
	f, err := coreio.Local.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}
