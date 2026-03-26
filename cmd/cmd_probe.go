package cmd

import (
	"context"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"forge.lthn.ai/core/cli/pkg/cli"
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

	core.Print(cmd.OutOrStdout(), "Running %d capability probes against %s...", len(ml.CapabilityProbes), apiURL)
	results := ml.RunCapabilityProbes(ctx, backend)

	core.Print(cmd.OutOrStdout(), "")
	core.Print(cmd.OutOrStdout(), "Results: %.1f%% (%d/%d)", results.Accuracy, results.Correct, results.Total)

	for cat, data := range results.ByCategory {
		catAcc := 0.0
		if data.Total > 0 {
			catAcc = float64(data.Correct) / float64(data.Total) * 100
		}
		core.Print(cmd.OutOrStdout(), "  %-20s %d/%d (%.0f%%)", cat, data.Correct, data.Total, catAcc)
	}

	if probeOutput != "" {
		if err := coreio.Local.Write(probeOutput, core.JSONMarshalString(results)); err != nil {
			return coreerr.E("cmd.runProbe", "write output", err)
		}
		core.Print(cmd.OutOrStdout(), "")
		core.Print(cmd.OutOrStdout(), "Results written to %s", probeOutput)
	}

	return nil
}
