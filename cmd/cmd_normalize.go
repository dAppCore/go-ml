package cmd

import (
	"dappco.re/go/core"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"dappco.re/go/core/store"
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
		path = core.Env("LEM_DB")
	}
	if path == "" {
		return coreerr.E("cmd.runNormalize", "--db or LEM_DB env is required", nil)
	}

	db, err := store.OpenDuckDBReadWrite(path)
	if err != nil {
		return coreerr.E("cmd.runNormalize", "open db", err)
	}
	defer db.Close()

	cfg := ml.NormalizeConfig{
		MinLength: normalizeMinLen,
	}

<<<<<<< HEAD
	return ml.NormalizeSeeds(db, cfg, nil)
=======
	return ml.NormalizeSeeds(db, cfg, cmd.OutOrStdout())
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
}
