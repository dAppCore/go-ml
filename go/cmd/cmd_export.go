package cmd

import (
	"dappco.re/go"
	coreio "dappco.re/go/io"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
	"dappco.re/go/store"
)

// addExportCommand registers `ml export` — reads golden set from DuckDB,
// filters, splits, and exports to training JSONL + optional Parquet.
//
//	core ml export --db lem.duckdb --output-dir ./train --train 80 --valid 10 --test 10
func addExportCommand(c *core.Core) {
	c.Command("ml/export", core.Command{
		Description: "Export golden set to training JSONL and Parquet",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			outputDir := opts.String("output-dir")
			if outputDir == "" {
				return resultFromError(coreerr.E("cmd.runExport", "--output-dir is required", nil))
			}
			minChars := optInt(opts, "min-chars", 50)
			trainPct := optInt(opts, "train", 80)
			validPct := optInt(opts, "valid", 10)
			testPct := optInt(opts, "test", 10)
			seed := int64(optInt(opts, "seed", 42))
			parquet := opts.Bool("parquet")

			if result := ml.ValidatePercentages(trainPct, validPct, testPct); !result.OK {
				return resultFromError(result)
			}

			if dbPath == "" {
				return resultFromError(coreerr.E("cmd.runExport", "--db or LEM_DB env is required", nil))
			}

			db, result := store.OpenDuckDB(dbPath)
			if !result.OK {
				return resultFromError(coreerr.E("cmd.runExport", "open db", errorFromResult(result)))
			}
			defer db.Close()

			rows, result := db.QueryGoldenSet(minChars)
			if !result.OK {
				return resultFromError(coreerr.E("cmd.runExport", "query golden set", errorFromResult(result)))
			}
			core.Print(nil, "Loaded %d golden set rows (min %d chars)", len(rows), minChars)

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
			core.Print(nil, "After filtering: %d responses", len(filtered))

			train, valid, test := ml.SplitData(filtered, trainPct, validPct, testPct, seed)
			core.Print(nil, "Split: train=%d, valid=%d, test=%d", len(train), len(valid), len(test))

			if err := coreio.Local.EnsureDir(outputDir); err != nil {
				return resultFromError(coreerr.E("cmd.runExport", "create output dir", err))
			}

			for _, split := range []struct {
				name string
				data []ml.Response
			}{
				{"train", train},
				{"valid", valid},
				{"test", test},
			} {
				path := core.JoinPath(outputDir, core.Concat(split.name, ".jsonl"))
				if result := ml.WriteTrainingJSONL(path, split.data); !result.OK {
					return resultFromError(coreerr.E("cmd.runExport", core.Sprintf("write %s", split.name), errorFromResult(result)))
				}
				core.Print(nil, "  %s.jsonl: %d examples", split.name, len(split.data))
			}

			if parquet {
				n, result := store.ExportParquet(outputDir, "")
				if !result.OK {
					return resultFromError(coreerr.E("cmd.runExport", "export parquet", errorFromResult(result)))
				}
				core.Print(nil, "  Parquet: %d total rows", n)
			}

			return core.Result{OK: true}
		},
	})
}
