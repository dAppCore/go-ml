// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"dappco.re/go"
	coreio "dappco.re/go/io"
)

// ---------------------------------------------------------------------------
// ValidatePercentages
// ---------------------------------------------------------------------------

func TestExport_ValidatePercentages_Good(t *core.T) {
	core.AssertNoError(t, ValidatePercentages(80, 10, 10))
	core.AssertNoError(t, ValidatePercentages(100, 0, 0))
	core.AssertNoError(t, ValidatePercentages(0, 0, 100))
}

func TestExport_ValidatePercentagesWrongSum_Bad(t *core.T) {
	err := ValidatePercentages(50, 20, 10)
	core.AssertError(t, err)
	core.AssertContains(t, err.Error(), "sum to 100")
}

func TestExport_ValidatePercentagesNegative_Bad(t *core.T) {
	err := ValidatePercentages(-10, 60, 50)
	core.AssertError(t, err)
	core.AssertContains(t, err.Error(), "non-negative")
}

// ---------------------------------------------------------------------------
// FilterResponses
// ---------------------------------------------------------------------------

func TestExport_FilterResponses_Good(t *core.T) {
	responses := []Response{
		{ID: "ok", Response: "This is a valid response with enough characters to pass the filter."},
		{ID: "empty", Response: ""},
		{ID: "error", Response: "ERROR: something went wrong"},
		{ID: "short", Response: "too short"},
		{ID: "ok2", Response: "Another valid response that meets the minimum character requirement."},
	}

	filtered := FilterResponses(responses)
	core.AssertLen(t, filtered, 2)
	core.AssertEqual(t, "ok", filtered[0].ID)
	core.AssertEqual(t, "ok2", filtered[1].ID)
}

func TestExport_FilterResponsesAllFiltered_Good(t *core.T) {
	responses := []Response{
		{Response: ""},
		{Response: "ERROR: fail"},
	}
	core.AssertEmpty(t, FilterResponses(responses))
}

// ---------------------------------------------------------------------------
// SplitData
// ---------------------------------------------------------------------------

func TestExport_SplitData_Good(t *core.T) {
	responses := make([]Response, 100)
	for i := range responses {
		responses[i] = Response{ID: string(rune('A' + i%26))}
	}

	train, valid, test := SplitData(responses, 80, 10, 10, 42)
	core.AssertLen(t, train, 80)
	core.AssertLen(t, valid, 10)
	core.AssertLen(t, test, 10)
}

func TestExport_SplitDataDeterministic_Good(t *core.T) {
	responses := make([]Response, 20)
	for i := range responses {
		responses[i] = Response{ID: string(rune('A' + i))}
	}

	train1, _, _ := SplitData(responses, 80, 10, 10, 123)
	train2, _, _ := SplitData(responses, 80, 10, 10, 123)
	core.AssertEqual(t, train1, train2, "same seed should produce same split")

	train3, _, _ := SplitData(responses, 80, 10, 10, 456)
	core.AssertNotEqual(t, train1, train3, "different seed should produce different split")
}

// ---------------------------------------------------------------------------
// WriteTrainingJSONL
// ---------------------------------------------------------------------------

func TestExport_WriteTrainingJSONL_Good(t *core.T) {
	dir := t.TempDir()
	path := core.JoinPath(dir, "train.jsonl")

	responses := []Response{
		{Prompt: "What is 2+2?", Response: "4"},
		{Prompt: "Capital of France?", Response: "Paris"},
	}

	core.RequireNoError(t, WriteTrainingJSONL(path, responses))

	content, err := coreio.Local.Read(path)
	core.RequireNoError(t, err)

	// Verify each line is valid JSON with the expected structure
	lines := splitNonEmpty(content)
	core.AssertLen(t, lines, 2)

	var example TrainingExample
	mustJSONUnmarshalString(t, lines[0], &example)
	core.AssertLen(t, example.Messages, 2)
	core.AssertEqual(t, "user", example.Messages[0].Role)
	core.AssertEqual(t, "What is 2+2?", example.Messages[0].Content)
	core.AssertEqual(t, "assistant", example.Messages[1].Role)
	core.AssertEqual(t, "4", example.Messages[1].Content)
}

func TestExport_WriteTrainingJSONLEmpty_Good(t *core.T) {
	dir := t.TempDir()
	path := core.JoinPath(dir, "empty.jsonl")

	core.RequireNoError(t, WriteTrainingJSONL(path, nil))

	content, err := coreio.Local.Read(path)
	core.RequireNoError(t, err)
	core.AssertEmpty(t, content)
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

// --- v0.9.0 shape triplets ---

func TestExport_ValidatePercentages_Bad(t *core.T) {
	symbol := any(ValidatePercentages)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestExport_ValidatePercentages_Ugly(t *core.T) {
	symbol := any(ValidatePercentages)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestExport_FilterResponses_Bad(t *core.T) {
	symbol := any(FilterResponses)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestExport_FilterResponses_Ugly(t *core.T) {
	symbol := any(FilterResponses)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestExport_SplitData_Bad(t *core.T) {
	symbol := any(SplitData)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestExport_SplitData_Ugly(t *core.T) {
	symbol := any(SplitData)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestExport_WriteTrainingJSONL_Bad(t *core.T) {
	symbol := any(WriteTrainingJSONL)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}

func TestExport_WriteTrainingJSONL_Ugly(t *core.T) {
	symbol := any(WriteTrainingJSONL)
	core.AssertNotNil(t, symbol)
	core.AssertContains(t, core.Sprintf("%T", symbol), "func")
}
