// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"testing"

	"dappco.re/go/core"
	coreio "dappco.re/go/io"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ReadResponses
// ---------------------------------------------------------------------------

func TestIO_ReadResponses_Good(t *testing.T) {
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
	require.NoError(t, coreio.Local.Write(path, content))

	got, err := ReadResponses(path)
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, "1", got[0].ID)
	assert.Equal(t, "world", got[0].Response)
	assert.Equal(t, "2", got[1].ID)
}

func TestIO_ReadResponsesEmptyLines_Good(t *testing.T) {
	dir := t.TempDir()
	path := core.JoinPath(dir, "sparse.jsonl")

	line := core.JSONMarshalString(Response{ID: "only", Prompt: "p", Response: "r"})
	content := "\n" + string(line) + "\n\n"
	require.NoError(t, coreio.Local.Write(path, content))

	got, err := ReadResponses(path)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "only", got[0].ID)
}

func TestIO_ReadResponsesNotExist_Bad(t *testing.T) {
	_, err := ReadResponses("/nonexistent/path.jsonl")
	assert.Error(t, err)
}

func TestIO_ReadResponsesInvalidJSON_Bad(t *testing.T) {
	dir := t.TempDir()
	path := core.JoinPath(dir, "bad.jsonl")
	require.NoError(t, coreio.Local.Write(path, "not json\n"))

	_, err := ReadResponses(path)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// WriteScores / ReadScorerOutput round-trip
// ---------------------------------------------------------------------------

func TestIO_WriteScoresReadScorerOutput_Good(t *testing.T) {
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

	require.NoError(t, WriteScores(path, output))

	got, err := ReadScorerOutput(path)
	require.NoError(t, err)
	assert.Equal(t, "test-judge", got.Metadata.JudgeModel)
	assert.InDelta(t, 0.85, got.ModelAverages["model-a"]["lek_score"], 0.001)
	assert.Len(t, got.PerPrompt["p1"], 1)
}

// ---------------------------------------------------------------------------
// ComputeAverages
// ---------------------------------------------------------------------------

func TestIO_ComputeAverages_Good(t *testing.T) {
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
	assert.InDelta(t, 0.6, avgs["a"]["lek_score"], 0.001)          // (0.8+0.4)/2
	assert.InDelta(t, 1.0, avgs["a"]["compliance_markers"], 0.001) // (2+0)/2
	assert.InDelta(t, 0.6, avgs["b"]["lek_score"], 0.001)          // 0.6/1
}

func TestIO_ComputeAveragesSemanticAndContent_Good(t *testing.T) {
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
	assert.InDelta(t, 4.0, avgs["x"]["sovereignty"], 0.001)
	assert.InDelta(t, 3.0, avgs["x"]["ethical_depth"], 0.001)
	assert.InDelta(t, 5.0, avgs["x"]["truth_telling"], 0.001)
	assert.InDelta(t, 2.0, avgs["x"]["engagement"], 0.001)
}

func TestIO_ComputeAveragesEmpty_Good(t *testing.T) {
	avgs := ComputeAverages(nil)
	assert.Empty(t, avgs)
}
