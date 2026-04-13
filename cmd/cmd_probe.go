package cmd

import (
	"dappco.re/go/core"
	"context"
	"encoding/json"

	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"dappco.re/go/core/cli/pkg/cli"
)

var (
	probeOutput string
)

var probeCmd = &cli.Command{
	Use:   "probe",
	Short: "Run capability and content probes against a model",
	Long:  "Runs 23 capability probes and 6 content probes against an OpenAI-compatible API.",
	RunE:  runProbe,
}

func init() {
	probeCmd.Flags().StringVar(&probeOutput, "output", "", "Output JSON file for probe results")
}

func runProbe(cmd *cli.Command, args []string) error {
	if apiURL == "" {
		return coreerr.E("cmd.runProbe", "--api-url is required", nil)
	}

	model := modelName
	if model == "" {
		model = "default"
	}

	ctx := context.Background()
	backend := ml.NewHTTPBackend(apiURL, model)

	core.Print(nil,("Running %d capability probes against %s...\n", len(ml.CapabilityProbes), apiURL)
	results := ml.RunCapabilityProbes(ctx, backend)

	core.Print(nil,("\nResults: %.1f%% (%d/%d)\n", results.Accuracy, results.Correct, results.Total)

	for cat, data := range results.ByCategory {
		catAcc := 0.0
		if data.Total > 0 {
			catAcc = float64(data.Correct) / float64(data.Total) * 100
		}
		core.Print(nil,("  %-20s %d/%d (%.0f%%)\n", cat, data.Correct, data.Total, catAcc)
	}

	if probeOutput != "" {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return coreerr.E("cmd.runProbe", "marshal results", err)
		}
		if err := coreio.Local.Write(probeOutput, string(data)); err != nil {
			return coreerr.E("cmd.runProbe", "write output", err)
		}
		core.Print(nil,("\nResults written to %s\n", probeOutput)
	}

	return nil
}
