package cmd

import (
	"os"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"dappco.re/go/core/cli/pkg/cli"
)

var normalizeMinLen int

var normalizeCmd = &cli.Command{
	Use:   "normalize",
	Short: "Normalize seeds into expansion prompts",
	Long:  "Deduplicates seeds against golden_set and prompts, creating the expansion_prompts table with priority-based ordering.",
	RunE:  runNormalize,
}

func init() {
	normalizeCmd.Flags().IntVar(&normalizeMinLen, "min-length", 50, "Minimum prompt length in characters")
}

func runNormalize(cmd *cli.Command, args []string) error {
	path := dbPath
	if path == "" {
		path = os.Getenv("LEM_DB")
	}
	if path == "" {
		return coreerr.E("cmd.runNormalize", "--db or LEM_DB env is required", nil)
	}

	db, err := ml.OpenDBReadWrite(path)
	if err != nil {
		return coreerr.E("cmd.runNormalize", "open db", err)
	}
	defer db.Close()

	cfg := ml.NormalizeConfig{
		MinLength: normalizeMinLen,
	}

	return ml.NormalizeSeeds(db, cfg, os.Stdout)
}
