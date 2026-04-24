// SPDX-License-Identifier: EUPL-1.2

package cmd

import (
	"context"

	"dappco.re/go/core"
	coreio "dappco.re/go/io"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
)

// addEvaluateCommand registers `ml evaluate` — the benchmark evaluator
// described in RFC §9. Given a model endpoint and a responses file, it
// runs the full five-suite scoring engine (heuristic, semantic, content,
// standard, exact) and writes the results. Used by LEM reference runs,
// OpenBrain RAG checks, and uptelligence baselines.
//
//	core ml evaluate --api-url http://localhost:11434 --model gemma3:27b \
//	    --input responses.jsonl --output evaluation.json
//	core ml evaluate --input responses.jsonl --suites heuristic,exact
func addEvaluateCommand(c *core.Core) {
	c.Command("ml/evaluate", core.Command{
		Description: "Evaluate a model by scoring responses against benchmarks",
		Action: func(opts core.Options) core.Result {
			readPersistentFlags(opts)

			input := opts.String("input")
			if input == "" {
				return resultFromError(coreerr.E("cmd.runEvaluate", "--input is required", nil))
			}
			output := opts.String("output")
			suites := opts.String("suites")
			if suites == "" {
				suites = "all"
			}
			concurrency := optInt(opts, "concurrency", 4)

			raw, err := coreio.Local.Read(input)
			if err != nil {
				return resultFromError(coreerr.E("cmd.runEvaluate", "read input", err))
			}

			responses, err := decodeResponsesJSONL(string(raw))
			if err != nil {
				return resultFromError(coreerr.E("cmd.runEvaluate", "decode jsonl", err))
			}

			// Judge is optional — heuristic/exact suites need no judge.
			var judge *ml.Judge
			if judgeURL != "" && judgeModel != "" {
				judgeBackend := ml.NewHTTPBackend(judgeURL, judgeModel)
				judge = ml.NewJudge(judgeBackend)
			}

			engine := ml.NewEngine(judge, concurrency, suites)

			core.Print(nil, "Evaluating %d responses with suites=%v concurrency=%d",
				len(responses), engine.SuiteNames(), concurrency)

			ctx := context.Background()
			results := engine.ScoreAll(ctx, responses)

			total := 0
			for _, scores := range results {
				total += len(scores)
			}
			core.Print(nil, "Scored %d prompts across %d models", total, len(results))

			if output != "" {
				if err := coreio.Local.Write(output, core.JSONMarshalString(results)); err != nil {
					return resultFromError(coreerr.E("cmd.runEvaluate", "write output", err))
				}
				core.Print(nil, "Results written to %s", output)
			}

			return core.Result{OK: true}
		},
	})
}

// decodeResponsesJSONL parses one ml.Response per non-empty line.
// Blank lines and lines beginning with '#' are skipped so the file can
// contain comments. Returns a slice plus any fatal parse error.
//
//	responses, err := decodeResponsesJSONL(string(fileBytes))
func decodeResponsesJSONL(text string) ([]ml.Response, error) {
	var out []ml.Response
	for _, line := range core.Split(text, "\n") {
		trim := core.Trim(line)
		if trim == "" || core.HasPrefix(trim, "#") {
			continue
		}
		var r ml.Response
		if res := core.JSONUnmarshalString(trim, &r); !res.OK {
			return nil, coreerr.E("cmd.decodeResponsesJSONL",
				core.Sprintf("parse line %q", truncateLine(trim, 120)), res.Value.(error))
		}
		out = append(out, r)
	}
	return out, nil
}

// truncateLine keeps error messages tidy when the offending JSONL row is huge.
//
//	truncateLine("0123456789", 4) // "0123…"
func truncateLine(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
