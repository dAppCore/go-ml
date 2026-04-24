package cmd

import (
	"context"
	"maps"
	"slices"
	"time"

	"dappco.re/go/core"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
)

// addScoreCommand registers `ml score` — reads a JSONL file of
// prompt/response pairs and scores them across configured suites.
//
//	core ml score --input responses.jsonl --suites all --output scores.json
func addScoreCommand(c *core.Core) {
	c.Command("ml/score", core.Command{
		Description: "Score responses with heuristic and LLM judges",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			input := opts.String("input")
			if input == "" {
				return resultFromError(coreerr.E("cmd.runScore", "--input is required", nil))
			}
			suites := optStringOr(opts, "suites", "all")
			output := opts.String("output")
			concurrency := optInt(opts, "concurrency", 4)

			responses, err := ml.ReadResponses(input)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runScore", "read input", err))
			}

			var judge *ml.Judge
			if judgeURL != "" {
				backend := ml.NewHTTPBackend(judgeURL, judgeModel)
				judge = ml.NewJudge(backend)
			}

			engine := ml.NewEngine(judge, concurrency, suites)

			ctx := context.Background()
			perPrompt := engine.ScoreAll(ctx, responses)
			averages := ml.ComputeAverages(perPrompt)

			if output != "" {
				out := &ml.ScorerOutput{
					Metadata: ml.Metadata{
						JudgeModel: judgeModel,
						JudgeURL:   judgeURL,
						ScoredAt:   time.Now(),
						Suites:     engine.SuiteNames(),
					},
					ModelAverages: averages,
					PerPrompt:     perPrompt,
				}
				if err := ml.WriteScores(output, out); err != nil {
					return resultFromError(coreerr.E("cmd.runScore", "write output", err))
				}
				core.Print(nil, "Scores written to %s", output)
			} else {
				for _, model := range slices.Sorted(maps.Keys(averages)) {
					avgs := averages[model]
					core.Print(nil, "%s:", model)
					for _, field := range slices.Sorted(maps.Keys(avgs)) {
						core.Print(nil, "  %-25s %.3f", field, avgs[field])
					}
				}
			}

			return core.Result{OK: true}
		},
	})
}
