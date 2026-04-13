package cmd

import (
	"dappco.re/go/core"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"dappco.re/go/core/store"
	"dappco.re/go/core/cli/pkg/cli"
)

var (
	approveOutput    string
	approveThreshold float64
)

var approveCmd = &cli.Command{
	Use:   "approve",
	Short: "Filter scored expansions into training JSONL",
	Long:  "Filters scored expansion responses by quality threshold and exports approved ones as chat-format training JSONL.",
	RunE:  runApprove,
}

func init() {
	approveCmd.Flags().StringVar(&approveOutput, "output", "", "Output JSONL file (defaults to expansion-approved.jsonl in db dir)")
	approveCmd.Flags().Float64Var(&approveThreshold, "threshold", 6.0, "Min judge average to approve")
}

func runApprove(cmd *cli.Command, args []string) error {
	path := dbPath
	if path == "" {
		path = core.Env("LEM_DB")
	}
	if path == "" {
		return coreerr.E("cmd.runApprove", "--db or LEM_DB required", nil)
	}

	output := approveOutput
	if output == "" {
<<<<<<< HEAD
		output = core.Path(core.PathDir(path), "expansion-approved.jsonl")
=======
		output = core.JoinPath(core.PathDir(path), "expansion-approved.jsonl")
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	}

	db, err := store.OpenDuckDB(path)
	if err != nil {
		return coreerr.E("cmd.runApprove", "open db", err)
	}
	defer db.Close()

	return ml.ApproveExpansions(db, ml.ApproveConfig{
		Output:    output,
		Threshold: approveThreshold,
	}, cmd.OutOrStdout())
}
