package cmd

import (
	"context"

	"dappco.re/go"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
	"dappco.re/go/store"
)

// addExpandCommand registers `ml expand` — reads pending expansion prompts
// from DuckDB and generates responses via an OpenAI-compatible API.
//
//	core ml expand --model gemma3:27b --limit 100 --output ./jsonl
func addExpandCommand(c *core.Core) {
	c.Command("ml/expand", core.Command{
		Description: "Generate expansion responses from pending prompts",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			if modelName == "" {
				return resultFromError(coreerr.E("cmd.runExpand", "--model is required", nil))
			}
			if dbPath == "" {
				return resultFromError(coreerr.E("cmd.runExpand", "--db or LEM_DB env is required", nil))
			}

			worker := opts.String("worker")
			if worker == "" {
				worker = core.Env("HOSTNAME")
			}
			output := optStringOr(opts, "output", ".")
			limit := opts.Int("limit")
			dryRun := opts.Bool("dry-run")

			db, err := store.OpenDuckDBReadWrite(dbPath)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runExpand", "open db", err))
			}
			defer db.Close()

			rows, err := db.QueryExpansionPrompts("pending", limit)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runExpand", "query expansion_prompts", err))
			}
			core.Print(nil, "Loaded %d pending prompts from %s", len(rows), dbPath)

			var prompts []ml.Response
			for _, r := range rows {
				prompt := r.Prompt
				if prompt == "" && r.PromptEn != "" {
					prompt = r.PromptEn
				}
				prompts = append(prompts, ml.Response{
					ID:     r.SeedID,
					Domain: r.Domain,
					Prompt: prompt,
				})
			}

			ctx := context.Background()
			backend := ml.NewHTTPBackend(apiURL, modelName)
			influx := ml.NewInfluxClient(influxURL, influxDB)

			return resultFromError(ml.ExpandPrompts(ctx, backend, influx, prompts, modelName, worker, output, dryRun, limit))
		},
	})
}
