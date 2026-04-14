package cmd

import (
	"context"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
)

// addProbeCommand registers `ml probe` — runs 23 capability probes and 6
// content probes against an OpenAI-compatible API.
//
//	core ml probe --api-url http://localhost:8090 --model gemma3:27b --output probes.json
func addProbeCommand(c *core.Core) {
	c.Command("ml/probe", core.Command{
		Description: "Run capability and content probes against a model",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			if apiURL == "" {
				return resultFromError(coreerr.E("cmd.runProbe", "--api-url is required", nil))
			}

			model := modelName
			if model == "" {
				model = "default"
			}
			output := opts.String("output")

			ctx := context.Background()
			backend := ml.NewHTTPBackend(apiURL, model)

			core.Print(nil, "Running %d capability probes against %s...", len(ml.CapabilityProbes), apiURL)
			results := ml.RunCapabilityProbes(ctx, backend)

			core.Print(nil, "")
			core.Print(nil, "Results: %.1f%% (%d/%d)", results.Accuracy, results.Correct, results.Total)

			for cat, data := range results.ByCategory {
				catAcc := 0.0
				if data.Total > 0 {
					catAcc = float64(data.Correct) / float64(data.Total) * 100
				}
				core.Print(nil, "  %-20s %d/%d (%.0f%%)", cat, data.Correct, data.Total, catAcc)
			}

			if output != "" {
				if err := coreio.Local.Write(output, core.JSONMarshalString(results)); err != nil {
					return resultFromError(coreerr.E("cmd.runProbe", "write output", err))
				}
				core.Print(nil, "")
				core.Print(nil, "Results written to %s", output)
			}

			return core.Result{OK: true}
		},
	})
}
