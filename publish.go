package ml

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
)

// PublishConfig holds options for the publish operation.
type PublishConfig struct {
	InputDir string
	Repo     string
	Public   bool
	Token    string
	DryRun   bool
}

// uploadEntry pairs a local file path with its remote destination.
type uploadEntry struct {
	local  string
	remote string
}

// Publish uploads Parquet files to HuggingFace Hub.
//
// It looks for train.parquet, valid.parquet, and test.parquet in InputDir,
// plus an optional dataset_card.md in the parent directory (uploaded as README.md).
// The token is resolved from PublishConfig.Token, the HF_TOKEN environment variable,
// or ~/.huggingface/token, in that order.
func Publish(cfg PublishConfig, w io.Writer) error {
	if cfg.InputDir == "" {
		return coreerr.E("ml.Publish", "input directory is required", nil)
	}

	token := resolveHFToken(cfg.Token)
	if token == "" && !cfg.DryRun {
		return coreerr.E("ml.Publish", "HuggingFace token required (--token, HF_TOKEN env, or ~/.huggingface/token)", nil)
	}

	files, err := collectUploadFiles(cfg.InputDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return coreerr.E("ml.Publish", fmt.Sprintf("no Parquet files found in %s", cfg.InputDir), nil)
	}

	if cfg.DryRun {
		fmt.Fprintf(w, "Dry run: would publish to %s\n", cfg.Repo)
		if cfg.Public {
			fmt.Fprintln(w, "  Visibility: public")
		} else {
			fmt.Fprintln(w, "  Visibility: private")
		}
		for _, f := range files {
			info, err := coreio.Local.Stat(f.local)
			if err != nil {
				return coreerr.E("ml.Publish", fmt.Sprintf("stat %s", f.local), err)
			}
			sizeMB := float64(info.Size()) / 1024 / 1024
			fmt.Fprintf(w, "  %s -> %s (%.1f MB)\n", filepath.Base(f.local), f.remote, sizeMB)
		}
		return nil
	}

	fmt.Fprintf(w, "Publishing to https://huggingface.co/datasets/%s\n", cfg.Repo)

	for _, f := range files {
		if err := uploadFileToHF(token, cfg.Repo, f.local, f.remote); err != nil {
			return coreerr.E("ml.Publish", fmt.Sprintf("upload %s", filepath.Base(f.local)), err)
		}
		fmt.Fprintf(w, "  Uploaded %s -> %s\n", filepath.Base(f.local), f.remote)
	}

	fmt.Fprintf(w, "\nPublished to https://huggingface.co/datasets/%s\n", cfg.Repo)
	return nil
}

// resolveHFToken returns a HuggingFace API token from the given value,
// HF_TOKEN env var, or ~/.huggingface/token file.
func resolveHFToken(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv("HF_TOKEN"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := coreio.Local.Read(filepath.Join(home, ".huggingface", "token"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// collectUploadFiles finds Parquet split files and an optional dataset card.
func collectUploadFiles(inputDir string) ([]uploadEntry, error) {
	splits := []string{"train", "valid", "test"}
	var files []uploadEntry

	for _, split := range splits {
		path := filepath.Join(inputDir, split+".parquet")
		if !coreio.Local.IsFile(path) {
			continue
		}
		files = append(files, uploadEntry{path, fmt.Sprintf("data/%s.parquet", split)})
	}

	// Check for dataset card in parent directory.
	cardPath := filepath.Join(inputDir, "..", "dataset_card.md")
	if coreio.Local.IsFile(cardPath) {
		files = append(files, uploadEntry{cardPath, "README.md"})
	}

	return files, nil
}

// uploadFileToHF uploads a single file to a HuggingFace dataset repo via the Hub API.
func uploadFileToHF(token, repoID, localPath, remotePath string) error {
	raw, err := coreio.Local.Read(localPath)
	if err != nil {
		return coreerr.E("ml.uploadFileToHF", fmt.Sprintf("read %s", localPath), err)
	}

	url := fmt.Sprintf("https://huggingface.co/api/datasets/%s/upload/main/%s", repoID, remotePath)

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader([]byte(raw)))
	if err != nil {
		return coreerr.E("ml.uploadFileToHF", "create request", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return coreerr.E("ml.uploadFileToHF", "upload request", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return coreerr.E("ml.uploadFileToHF", fmt.Sprintf("upload failed: HTTP %d: %s", resp.StatusCode, string(body)), nil)
	}

	return nil
}
