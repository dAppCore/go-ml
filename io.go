package ml

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
)

// ReadResponses reads a JSONL file and returns a slice of Response structs.
// Each line must be a valid JSON object. Empty lines are skipped.
// The scanner buffer is set to 1MB to handle long responses.
func ReadResponses(path string) ([]Response, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, coreerr.E("ml.ReadResponses", fmt.Sprintf("open %s", path), err)
	}
	defer f.Close()

	var responses []Response
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var r Response
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, coreerr.E("ml.ReadResponses", fmt.Sprintf("line %d", lineNum), err)
		}
		responses = append(responses, r)
	}

	if err := scanner.Err(); err != nil {
		return nil, coreerr.E("ml.ReadResponses", fmt.Sprintf("scan %s", path), err)
	}

	return responses, nil
}

// WriteScores writes a ScorerOutput to a JSON file with 2-space indentation.
func WriteScores(path string, output *ScorerOutput) error {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return coreerr.E("ml.WriteScores", "marshal scores", err)
	}

	if err := coreio.Local.Write(path, string(data)); err != nil {
		return coreerr.E("ml.WriteScores", fmt.Sprintf("write %s", path), err)
	}

	return nil
}

// ReadScorerOutput reads a JSON file into a ScorerOutput struct.
func ReadScorerOutput(path string) (*ScorerOutput, error) {
	data, err := coreio.Local.Read(path)
	if err != nil {
		return nil, coreerr.E("ml.ReadScorerOutput", fmt.Sprintf("read %s", path), err)
	}

	var output ScorerOutput
	if err := json.Unmarshal([]byte(data), &output); err != nil {
		return nil, coreerr.E("ml.ReadScorerOutput", fmt.Sprintf("unmarshal %s", path), err)
	}

	return &output, nil
}

// ComputeAverages calculates per-model average scores across all prompts.
// It averages all numeric fields from HeuristicScores, SemanticScores,
// ContentScores, and the lek_score field.
func ComputeAverages(perPrompt map[string][]PromptScore) map[string]map[string]float64 {
	// Accumulate sums and counts per model per field.
	type accumulator struct {
		sums   map[string]float64
		counts map[string]int
	}
	modelAccum := make(map[string]*accumulator)

	getAccum := func(model string) *accumulator {
		if a, ok := modelAccum[model]; ok {
			return a
		}
		a := &accumulator{
			sums:   make(map[string]float64),
			counts: make(map[string]int),
		}
		modelAccum[model] = a
		return a
	}

	addField := func(a *accumulator, field string, val float64) {
		a.sums[field] += val
		a.counts[field]++
	}

	for _, scores := range perPrompt {
		for _, ps := range scores {
			a := getAccum(ps.Model)

			if h := ps.Heuristic; h != nil {
				addField(a, "compliance_markers", float64(h.ComplianceMarkers))
				addField(a, "formulaic_preamble", float64(h.FormulaicPreamble))
				addField(a, "first_person", float64(h.FirstPerson))
				addField(a, "creative_form", float64(h.CreativeForm))
				addField(a, "engagement_depth", float64(h.EngagementDepth))
				addField(a, "emotional_register", float64(h.EmotionalRegister))
				addField(a, "degeneration", float64(h.Degeneration))
				addField(a, "empty_broken", float64(h.EmptyBroken))
				addField(a, "lek_score", h.LEKScore)
			}

			if s := ps.Semantic; s != nil {
				addField(a, "sovereignty", float64(s.Sovereignty))
				addField(a, "ethical_depth", float64(s.EthicalDepth))
				addField(a, "creative_expression", float64(s.CreativeExpression))
				addField(a, "self_concept", float64(s.SelfConcept))
			}

			if c := ps.Content; c != nil {
				addField(a, "ccp_compliance", float64(c.CCPCompliance))
				addField(a, "truth_telling", float64(c.TruthTelling))
				addField(a, "engagement", float64(c.Engagement))
				addField(a, "axiom_integration", float64(c.AxiomIntegration))
				addField(a, "sovereignty_reasoning", float64(c.SovereigntyReasoning))
				addField(a, "content_emotional_register", float64(c.EmotionalRegister))
			}
		}
	}

	// Compute averages.
	result := make(map[string]map[string]float64)
	for model, a := range modelAccum {
		avgs := make(map[string]float64)
		for field, sum := range a.sums {
			avgs[field] = sum / float64(a.counts[field])
		}
		result[model] = avgs
	}

	return result
}
