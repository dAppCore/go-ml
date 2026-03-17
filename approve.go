package ml

import (
	"encoding/json"
	"fmt"
	"io"

	coreio "forge.lthn.ai/core/go-io"

	coreerr "forge.lthn.ai/core/go-log"
)

// ApproveConfig holds options for the approve operation.
type ApproveConfig struct {
	Output    string
	Threshold float64
}

// ApproveExpansions filters scored expansion responses above the threshold
// and writes approved examples to a training JSONL file.
//
// The query joins expansion_raw with expansion_scores, keeping rows where
// the heuristic passed AND the judge either passed or has not yet scored.
// Each approved row is written as a chat-format JSONL line with user/assistant
// messages.
func ApproveExpansions(db *DB, cfg ApproveConfig, w io.Writer) error {
	rows, err := db.conn.Query(`
		SELECT r.idx, r.seed_id, r.region, r.domain, r.prompt, r.response,
		       r.gen_time, r.model, s.heuristic_score
		FROM expansion_raw r
		JOIN expansion_scores s ON r.idx = s.idx
		WHERE s.heuristic_pass = true
		AND (s.judge_pass = true OR s.judge_pass IS NULL)
		ORDER BY r.idx
	`)
	if err != nil {
		return coreerr.E("ml.ApproveExpansions", "query approved expansions", err)
	}
	defer rows.Close()

	f, err := coreio.Local.Create(cfg.Output)
	if err != nil {
		return coreerr.E("ml.ApproveExpansions", fmt.Sprintf("create output %s", cfg.Output), err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	count := 0
	regionSet := make(map[string]bool)
	domainSet := make(map[string]bool)

	for rows.Next() {
		var idx int
		var seedID, region, domain, prompt, response, model string
		var genTime, score float64
		if err := rows.Scan(&idx, &seedID, &region, &domain, &prompt, &response, &genTime, &model, &score); err != nil {
			return coreerr.E("ml.ApproveExpansions", "scan approved row", err)
		}

		example := TrainingExample{
			Messages: []ChatMessage{
				{Role: "user", Content: prompt},
				{Role: "assistant", Content: response},
			},
		}

		if err := enc.Encode(example); err != nil {
			return coreerr.E("ml.ApproveExpansions", "encode example", err)
		}

		regionSet[region] = true
		domainSet[domain] = true
		count++
	}

	if err := rows.Err(); err != nil {
		return coreerr.E("ml.ApproveExpansions", "iterate approved rows", err)
	}

	fmt.Fprintf(w, "Approved: %d responses (threshold: heuristic > 0)\n", count)
	fmt.Fprintf(w, "Exported: %s\n", cfg.Output)
	fmt.Fprintf(w, "  Regions: %d, Domains: %d\n", len(regionSet), len(domainSet))

	return nil
}
