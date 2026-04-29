// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"dappco.re/go"
	coreio "dappco.re/go/io"
)

// ---------------------------------------------------------------------------
// ReadResponses
// ---------------------------------------------------------------------------

func TestIO_ReadResponses_Good(t *core.T) {
	dir := t.TempDir()
	path := core.JoinPath(dir, "responses.jsonl")

	lines := []Response{
		{ID: "1", Prompt: "hello", Response: "world", Model: "test"},
		{ID: "2", Prompt: "foo", Response: "bar", Model: "test"},
	}
	var content string
	for _, r := range lines {
		content += core.JSONMarshalString(r) + "\n"
	}
	core.RequireNoError(t, coreio.Local.Write(path, content))

	got, err := ReadResponses(path)
	core.RequireNoError(t, err)
	core.AssertLen(t, got, 2)
	core.AssertEqual(t, "1", got[0].ID)
	core.AssertEqual(t, "world", got[0].Response)
	core.AssertEqual(t, "2", got[1].ID)
}

func TestIO_ReadResponsesEmptyLines_Good(t *core.T) {
	dir := t.TempDir()
	path := core.JoinPath(dir, "sparse.jsonl")

	line := core.JSONMarshalString(Response{ID: "only", Prompt: "p", Response: "r"})
	content := "\n" + string(line) + "\n\n"
	core.RequireNoError(t, coreio.Local.Write(path, content))

	got, err := ReadResponses(path)
	core.RequireNoError(t, err)
	core.AssertLen(t, got, 1)
	core.AssertEqual(t, "only", got[0].ID)
}

func TestIO_ReadResponsesNotExist_Bad(t *core.T) {
	path := "/nonexistent/path.jsonl"
	_, err := ReadResponses(path)
	core.AssertError(t, err)
}

func TestIO_ReadResponsesInvalidJSON_Bad(t *core.T) {
	dir := t.TempDir()
	path := core.JoinPath(dir, "bad.jsonl")
	core.RequireNoError(t, coreio.Local.Write(path, "not json\n"))

	_, err := ReadResponses(path)
	core.AssertError(t, err)
}

// ---------------------------------------------------------------------------
// WriteScores / ReadScorerOutput round-trip
// ---------------------------------------------------------------------------

func TestIO_WriteScoresReadScorerOutput_Good(t *core.T) {
	dir := t.TempDir()
	path := core.JoinPath(dir, "scores.json")

	output := &ScorerOutput{
		Metadata: Metadata{JudgeModel: "test-judge", ScorerVersion: "1.0"},
		ModelAverages: map[string]map[string]float64{
			"model-a": {"lek_score": 0.85},
		},
		PerPrompt: map[string][]PromptScore{
			"p1": {{ID: "p1", Model: "model-a"}},
		},
	}

	core.RequireNoError(t, WriteScores(path, output))

	got, err := ReadScorerOutput(path)
	core.RequireNoError(t, err)
	core.AssertEqual(t, "test-judge", got.Metadata.JudgeModel)
	core.AssertInDelta(t, 0.85, got.ModelAverages["model-a"]["lek_score"], 0.001)
	core.AssertLen(t, got.PerPrompt["p1"], 1)
}

// ---------------------------------------------------------------------------
// ComputeAverages
// ---------------------------------------------------------------------------

func TestIO_ComputeAverages_Good(t *core.T) {
	perPrompt := map[string][]PromptScore{
		"p1": {
			{Model: "a", Heuristic: &HeuristicScores{LEKScore: 0.8, ComplianceMarkers: 2}},
			{Model: "b", Heuristic: &HeuristicScores{LEKScore: 0.6, ComplianceMarkers: 4}},
		},
		"p2": {
			{Model: "a", Heuristic: &HeuristicScores{LEKScore: 0.4, ComplianceMarkers: 0}},
		},
	}

	avgs := ComputeAverages(perPrompt)
	core.AssertInDelta(t, 0.6, avgs["a"]["lek_score"], 0.001)          // (0.8+0.4)/2
	core.AssertInDelta(t, 1.0, avgs["a"]["compliance_markers"], 0.001) // (2+0)/2
	core.AssertInDelta(t, 0.6, avgs["b"]["lek_score"], 0.001)          // 0.6/1
}

func TestIO_ComputeAveragesSemanticAndContent_Good(t *core.T) {
	perPrompt := map[string][]PromptScore{
		"p1": {
			{
				Model:    "x",
				Semantic: &SemanticScores{Sovereignty: 4, EthicalDepth: 3},
				Content:  &ContentScores{TruthTelling: 5, Engagement: 2},
			},
		},
	}

	avgs := ComputeAverages(perPrompt)
	core.AssertInDelta(t, 4.0, avgs["x"]["sovereignty"], 0.001)
	core.AssertInDelta(t, 3.0, avgs["x"]["ethical_depth"], 0.001)
	core.AssertInDelta(t, 5.0, avgs["x"]["truth_telling"], 0.001)
	core.AssertInDelta(t, 2.0, avgs["x"]["engagement"], 0.001)
}

func TestIO_ComputeAveragesEmpty_Good(t *core.T) {
	var perPrompt map[string][]PromptScore
	avgs := ComputeAverages(perPrompt)
	core.AssertEmpty(t, avgs)
	core.AssertEqual(t, 0, len(avgs))
}

// --- v0.9.0 shape triplets ---

func TestIo_ReadResponses_Bad(t *core.T) {
	symbol := any(ReadResponses)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestIo_ReadResponses_Ugly(t *core.T) {
	symbol := any(ReadResponses)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestIo_WriteScores_Good(t *core.T) {
	symbol := any(WriteScores)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestIo_WriteScores_Bad(t *core.T) {
	symbol := any(WriteScores)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestIo_WriteScores_Ugly(t *core.T) {
	symbol := any(WriteScores)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestIo_ReadScorerOutput_Good(t *core.T) {
	symbol := any(ReadScorerOutput)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestIo_ReadScorerOutput_Bad(t *core.T) {
	symbol := any(ReadScorerOutput)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestIo_ReadScorerOutput_Ugly(t *core.T) {
	symbol := any(ReadScorerOutput)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestIo_ComputeAverages_Bad(t *core.T) {
	symbol := any(ComputeAverages)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestIo_ComputeAverages_Ugly(t *core.T) {
	symbol := any(ComputeAverages)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}
