package cmd

import (
	"os"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"dappco.re/go/core/cli/pkg/cli"
)

var coverageCmd = &cli.Command{
	Use:   "coverage",
	Short: "Analyze seed coverage by region and domain",
	Long:  "Queries seeds by region and domain, renders ASCII bar charts, and highlights underrepresented areas.",
	RunE:  runCoverage,
}

func runCoverage(cmd *cli.Command, args []string) error {
	path := dbPath
	if path == "" {
		path = os.Getenv("LEM_DB")
	}
	if path == "" {
		return coreerr.E("cmd.runCoverage", "--db or LEM_DB required", nil)
	}

	db, err := ml.OpenDB(path)
	if err != nil {
		return coreerr.E("cmd.runCoverage", "open db", err)
	}
	defer db.Close()

	return ml.PrintCoverage(db, cmd.OutOrStdout())
}
