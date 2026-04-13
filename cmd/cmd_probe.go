package cmd

import (
	"dappco.re/go/core"
	"context"
<<<<<<< HEAD
	"encoding/json"
=======
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

	"dappco.re/go/core"
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

<<<<<<< HEAD
	core.Print(nil,("Running %d capability probes against %s...\n", len(ml.CapabilityProbes), apiURL)
	results := ml.RunCapabilityProbes(ctx, backend)

	core.Print(nil,("\nResults: %.1f%% (%d/%d)\n", results.Accuracy, results.Correct, results.Total)
=======
	core.Print(cmd.OutOrStdout(), "Running %d capability probes against %s...", len(ml.CapabilityProbes), apiURL)
	results := ml.RunCapabilityProbes(ctx, backend)

	core.Print(cmd.OutOrStdout(), "")
	core.Print(cmd.OutOrStdout(), "Results: %.1f%% (%d/%d)", results.Accuracy, results.Correct, results.Total)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

	for cat, data := range results.ByCategory {
		catAcc := 0.0
		if data.Total > 0 {
			catAcc = float64(data.Correct) / float64(data.Total) * 100
		}
<<<<<<< HEAD
		core.Print(nil,("  %-20s %d/%d (%.0f%%)\n", cat, data.Correct, data.Total, catAcc)
=======
		core.Print(cmd.OutOrStdout(), "  %-20s %d/%d (%.0f%%)", cat, data.Correct, data.Total, catAcc)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	}

	if probeOutput != "" {
		if err := coreio.Local.Write(probeOutput, core.JSONMarshalString(results)); err != nil {
			return coreerr.E("cmd.runProbe", "write output", err)
		}
<<<<<<< HEAD
		core.Print(nil,("\nResults written to %s\n", probeOutput)
=======
		core.Print(cmd.OutOrStdout(), "")
		core.Print(cmd.OutOrStdout(), "Results written to %s", probeOutput)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	}

	return nil
}
