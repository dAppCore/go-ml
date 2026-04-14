// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"testing"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ValidatePercentages
// ---------------------------------------------------------------------------

func TestExport_ValidatePercentages_Good(t *testing.T) {
	assert.NoError(t, ValidatePercentages(80, 10, 10))
	assert.NoError(t, ValidatePercentages(100, 0, 0))
	assert.NoError(t, ValidatePercentages(0, 0, 100))
}

func TestExport_ValidatePercentagesWrongSum_Bad(t *testing.T) {
	err := ValidatePercentages(50, 20, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sum to 100")
}

func TestExport_ValidatePercentagesNegative_Bad(t *testing.T) {
	err := ValidatePercentages(-10, 60, 50)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-negative")
}

// ---------------------------------------------------------------------------
// FilterResponses
// ---------------------------------------------------------------------------

func TestExport_FilterResponses_Good(t *testing.T) {
	responses := []Response{
		{ID: "ok", Response: "This is a valid response with enough characters to pass the filter."},
		{ID: "empty", Response: ""},
		{ID: "error", Response: "ERROR: something went wrong"},
		{ID: "short", Response: "too short"},
		{ID: "ok2", Response: "Another valid response that meets the minimum character requirement."},
	}

	filtered := FilterResponses(responses)
	assert.Len(t, filtered, 2)
	assert.Equal(t, "ok", filtered[0].ID)
	assert.Equal(t, "ok2", filtered[1].ID)
}

func TestExport_FilterResponsesAllFiltered_Good(t *testing.T) {
	responses := []Response{
		{Response: ""},
		{Response: "ERROR: fail"},
	}
	assert.Empty(t, FilterResponses(responses))
}

// ---------------------------------------------------------------------------
// SplitData
// ---------------------------------------------------------------------------

func TestExport_SplitData_Good(t *testing.T) {
	responses := make([]Response, 100)
	for i := range responses {
		responses[i] = Response{ID: string(rune('A' + i%26))}
	}

	train, valid, test := SplitData(responses, 80, 10, 10, 42)
	assert.Len(t, train, 80)
	assert.Len(t, valid, 10)
	assert.Len(t, test, 10)
}

func TestExport_SplitDataDeterministic_Good(t *testing.T) {
	responses := make([]Response, 20)
	for i := range responses {
		responses[i] = Response{ID: string(rune('A' + i))}
	}

	train1, _, _ := SplitData(responses, 80, 10, 10, 123)
	train2, _, _ := SplitData(responses, 80, 10, 10, 123)
	assert.Equal(t, train1, train2, "same seed should produce same split")

	train3, _, _ := SplitData(responses, 80, 10, 10, 456)
	assert.NotEqual(t, train1, train3, "different seed should produce different split")
}

// ---------------------------------------------------------------------------
// WriteTrainingJSONL
// ---------------------------------------------------------------------------

func TestExport_WriteTrainingJSONL_Good(t *testing.T) {
	dir := t.TempDir()
	path := core.JoinPath(dir, "train.jsonl")

	responses := []Response{
		{Prompt: "What is 2+2?", Response: "4"},
		{Prompt: "Capital of France?", Response: "Paris"},
	}

	require.NoError(t, WriteTrainingJSONL(path, responses))

	content, err := coreio.Local.Read(path)
	require.NoError(t, err)

	// Verify each line is valid JSON with the expected structure
	lines := splitNonEmpty(content)
	assert.Len(t, lines, 2)

	var example TrainingExample
	mustJSONUnmarshalString(t, lines[0], &example)
	assert.Len(t, example.Messages, 2)
	assert.Equal(t, "user", example.Messages[0].Role)
	assert.Equal(t, "What is 2+2?", example.Messages[0].Content)
	assert.Equal(t, "assistant", example.Messages[1].Role)
	assert.Equal(t, "4", example.Messages[1].Content)
}

func TestExport_WriteTrainingJSONLEmpty_Good(t *testing.T) {
	dir := t.TempDir()
	path := core.JoinPath(dir, "empty.jsonl")

	require.NoError(t, WriteTrainingJSONL(path, nil))

	content, err := coreio.Local.Read(path)
	require.NoError(t, err)
	assert.Empty(t, content)
}

// splitNonEmpty splits a string by newlines and returns non-empty entries.
func splitNonEmpty(s string) []string {
	var result []string
	for _, line := range []byte(s) {
		_ = line
	}
	// Simple line split
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}
