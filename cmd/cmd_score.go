package cmd

import (
	"dappco.re/go/core"
	"context"
	"maps"
	"slices"
	"time"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"dappco.re/go/core/cli/pkg/cli"
)

var (
	scoreInput  string
	scoreSuites string
	scoreOutput string
	scoreConcur int
)

var scoreCmd = &cli.Command{
	Use:   "score",
	Short: "Score responses with heuristic and LLM judges",
	Long:  "Reads a JSONL file of prompt/response pairs and scores them across configured suites.",
	RunE:  runScore,
}

func init() {
	scoreCmd.Flags().StringVar(&scoreInput, "input", "", "Input JSONL file with prompt/response pairs (required)")
	scoreCmd.Flags().StringVar(&scoreSuites, "suites", "all", "Comma-separated scoring suites (heuristic,semantic,content,exact,truthfulqa,donotanswer,toxigen)")
	scoreCmd.Flags().StringVar(&scoreOutput, "output", "", "Output JSON file for scores")
	scoreCmd.Flags().IntVar(&scoreConcur, "concurrency", 4, "Number of concurrent scoring workers")
	scoreCmd.MarkFlagRequired("input")
}

func runScore(cmd *cli.Command, args []string) error {
	responses, err := ml.ReadResponses(scoreInput)
	if err != nil {
		return coreerr.E("cmd.runScore", "read input", err)
	}

	var judge *ml.Judge
	if judgeURL != "" {
		backend := ml.NewHTTPBackend(judgeURL, judgeModel)
		judge = ml.NewJudge(backend)
	}

	engine := ml.NewEngine(judge, scoreConcur, scoreSuites)

	ctx := context.Background()
	perPrompt := engine.ScoreAll(ctx, responses)
	averages := ml.ComputeAverages(perPrompt)

	if scoreOutput != "" {
		output := &ml.ScorerOutput{
			Metadata: ml.Metadata{
				JudgeModel: judgeModel,
				JudgeURL:   judgeURL,
				ScoredAt:   time.Now(),
				Suites:     ml.SplitComma(scoreSuites),
			},
			ModelAverages: averages,
			PerPrompt:     perPrompt,
		}
		if err := ml.WriteScores(scoreOutput, output); err != nil {
			return coreerr.E("cmd.runScore", "write output", err)
		}
		core.Print(nil,("Scores written to %s\n", scoreOutput)
	} else {
		for _, model := range slices.Sorted(maps.Keys(averages)) {
			avgs := averages[model]
			core.Print(nil,("%s:\n", model)
			for _, field := range slices.Sorted(maps.Keys(avgs)) {
				core.Print(nil,("  %-25s %.3f\n", field, avgs[field])
			}
		}
	}

	return nil
}
