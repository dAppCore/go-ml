package cmd

import (
	"fmt"
	"os"

	"forge.lthn.ai/core/cli/pkg/cli"
	"forge.lthn.ai/core/go-ml"

	coreio "forge.lthn.ai/core/go-io"

	coreerr "forge.lthn.ai/core/go-log"
)

var (
	exportOutputDir string
	exportMinChars  int
	exportTrainPct  int
	exportValidPct  int
	exportTestPct   int
	exportSeed      int64
	exportParquet   bool
)

var exportCmd = &cli.Command{
	Use:   "export",
	Short: "Export golden set to training JSONL and Parquet",
	Long:  "Reads golden set from DuckDB, filters, splits, and exports to JSONL and optionally Parquet.",
	RunE:  runExport,
}

func init() {
	exportCmd.Flags().StringVar(&exportOutputDir, "output-dir", "", "Output directory for training files (required)")
	exportCmd.Flags().IntVar(&exportMinChars, "min-chars", 50, "Minimum response length in characters")
	exportCmd.Flags().IntVar(&exportTrainPct, "train", 80, "Training split percentage")
	exportCmd.Flags().IntVar(&exportValidPct, "valid", 10, "Validation split percentage")
	exportCmd.Flags().IntVar(&exportTestPct, "test", 10, "Test split percentage")
	exportCmd.Flags().Int64Var(&exportSeed, "seed", 42, "Random seed for shuffle")
	exportCmd.Flags().BoolVar(&exportParquet, "parquet", false, "Also export Parquet files")
	exportCmd.MarkFlagRequired("output-dir")
}

func runExport(cmd *cli.Command, args []string) error {
	if err := ml.ValidatePercentages(exportTrainPct, exportValidPct, exportTestPct); err != nil {
		return err
	}

	path := dbPath
	if path == "" {
		path = os.Getenv("LEM_DB")
	}
	if path == "" {
		return coreerr.E("cmd.runExport", "--db or LEM_DB env is required", nil)
	}

	db, err := ml.OpenDB(path)
	if err != nil {
		return coreerr.E("cmd.runExport", "open db", err)
	}
	defer db.Close()

	rows, err := db.QueryGoldenSet(exportMinChars)
	if err != nil {
		return coreerr.E("cmd.runExport", "query golden set", err)
	}
	fmt.Printf("Loaded %d golden set rows (min %d chars)\n", len(rows), exportMinChars)

	// Convert to Response format.
	var responses []ml.Response
	for _, r := range rows {
		responses = append(responses, ml.Response{
			ID:       r.SeedID,
			Domain:   r.Domain,
			Prompt:   r.Prompt,
			Response: r.Response,
		})
	}

	filtered := ml.FilterResponses(responses)
	fmt.Printf("After filtering: %d responses\n", len(filtered))

	train, valid, test := ml.SplitData(filtered, exportTrainPct, exportValidPct, exportTestPct, exportSeed)
	fmt.Printf("Split: train=%d, valid=%d, test=%d\n", len(train), len(valid), len(test))

	if err := coreio.Local.EnsureDir(exportOutputDir); err != nil {
		return coreerr.E("cmd.runExport", "create output dir", err)
	}

	for _, split := range []struct {
		name string
		data []ml.Response
	}{
		{"train", train},
		{"valid", valid},
		{"test", test},
	} {
		path := fmt.Sprintf("%s/%s.jsonl", exportOutputDir, split.name)
		if err := ml.WriteTrainingJSONL(path, split.data); err != nil {
			return coreerr.E("cmd.runExport", fmt.Sprintf("write %s", split.name), err)
		}
		fmt.Printf("  %s.jsonl: %d examples\n", split.name, len(split.data))
	}

	if exportParquet {
		n, err := ml.ExportParquet(exportOutputDir, "")
		if err != nil {
			return coreerr.E("cmd.runExport", "export parquet", err)
		}
		fmt.Printf("  Parquet: %d total rows\n", n)
	}

	return nil
}
