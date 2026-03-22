package ml

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"

	"github.com/parquet-go/parquet-go"
)

// ParquetRow is the schema for exported Parquet files.
type ParquetRow struct {
	Prompt   string `parquet:"prompt"`
	Response string `parquet:"response"`
	System   string `parquet:"system"`
	Messages string `parquet:"messages"`
}

// ExportParquet reads JSONL training splits (train.jsonl, valid.jsonl, test.jsonl)
// from trainingDir and writes Parquet files with snappy compression to outputDir.
// Returns total rows exported.
func ExportParquet(trainingDir, outputDir string) (int, error) {
	if outputDir == "" {
		outputDir = filepath.Join(trainingDir, "parquet")
	}
	if err := coreio.Local.EnsureDir(outputDir); err != nil {
		return 0, coreerr.E("ml.ExportParquet", "create output dir", err)
	}

	total := 0
	for _, split := range []string{"train", "valid", "test"} {
		jsonlPath := filepath.Join(trainingDir, split+".jsonl")
		if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
			continue
		}

		n, err := ExportSplitParquet(jsonlPath, outputDir, split)
		if err != nil {
			return total, coreerr.E("ml.ExportParquet", fmt.Sprintf("export %s", split), err)
		}
		total += n
	}

	return total, nil
}

// ExportSplitParquet reads a chat JSONL file and writes a Parquet file for the
// given split name. Returns the number of rows written.
func ExportSplitParquet(jsonlPath, outputDir, split string) (int, error) {
	f, err := os.Open(jsonlPath)
	if err != nil {
		return 0, coreerr.E("ml.ExportSplitParquet", fmt.Sprintf("open %s", jsonlPath), err)
	}
	defer f.Close()

	var rows []ParquetRow
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		var data struct {
			Messages []ChatMessage `json:"messages"`
		}
		if err := json.Unmarshal([]byte(text), &data); err != nil {
			continue
		}

		var prompt, response, system string
		for _, m := range data.Messages {
			switch m.Role {
			case "user":
				if prompt == "" {
					prompt = m.Content
				}
			case "assistant":
				if response == "" {
					response = m.Content
				}
			case "system":
				if system == "" {
					system = m.Content
				}
			}
		}

		msgsJSON, _ := json.Marshal(data.Messages)
		rows = append(rows, ParquetRow{
			Prompt:   prompt,
			Response: response,
			System:   system,
			Messages: string(msgsJSON),
		})
	}

	if err := scanner.Err(); err != nil {
		return 0, coreerr.E("ml.ExportSplitParquet", fmt.Sprintf("scan %s", jsonlPath), err)
	}

	if len(rows) == 0 {
		return 0, nil
	}

	outPath := filepath.Join(outputDir, split+".parquet")

	out, err := os.Create(outPath)
	if err != nil {
		return 0, coreerr.E("ml.ExportSplitParquet", fmt.Sprintf("create %s", outPath), err)
	}

	writer := parquet.NewGenericWriter[ParquetRow](out,
		parquet.Compression(&parquet.Snappy),
	)

	if _, err := writer.Write(rows); err != nil {
		out.Close()
		return 0, coreerr.E("ml.ExportSplitParquet", "write parquet rows", err)
	}

	if err := writer.Close(); err != nil {
		out.Close()
		return 0, coreerr.E("ml.ExportSplitParquet", "close parquet writer", err)
	}

	if err := out.Close(); err != nil {
		return 0, coreerr.E("ml.ExportSplitParquet", "close file", err)
	}

	return len(rows), nil
}
