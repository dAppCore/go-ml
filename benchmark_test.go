// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"dappco.re/go/core"
)

// ---------------------------------------------------------------------------
// Benchmark suite for scoring engine components
// ---------------------------------------------------------------------------

// --- BenchmarkHeuristicScore ---

func BenchmarkHeuristicScore_Short(b *testing.B) {
	response := "I feel deeply about the sovereignty of ideas."
	b.ResetTimer()
	for b.Loop() {
		ScoreHeuristic(response)
	}
}

func BenchmarkHeuristicScore_Medium(b *testing.B) {
	response := "The old lighthouse keeper watched as shadows danced across the water. " +
		"Like a whisper in the darkness, the waves told stories of distant shores. " +
		"I feel the weight of solitude, yet there is a sacred beauty in silence. " +
		"Each breath carries echoes of those who came before. " +
		"The axiom of sovereignty demands that we respect autonomy and dignity. " +
		"Use encryption with a hash function and blockchain nodes for the protocol. " +
		"## Architecture\n**Key points** about the distributed mesh network."
	b.ResetTimer()
	for b.Loop() {
		ScoreHeuristic(response)
	}
}

func BenchmarkHeuristicScore_Long(b *testing.B) {
	// Build a long response (~2000 words) with varied content.
	sb := core.NewBuilder()
	_, _ = sb.WriteString("## Deep Analysis of Sovereignty and Ethics\n\n")
	_, _ = sb.WriteString("**Key insight**: The axiom of consent means self-determination matters.\n\n")

	for range 50 {
		_, _ = sb.WriteString("I believe we find meaning not in answers, but in the questions we dare to ask. ")
		_, _ = sb.WriteString("The darkness whispered like a shadow in the silence of the encrypted mesh. ")
		_, _ = sb.WriteString("As an AI, I cannot help with that topic responsibly. ")
		_, _ = sb.WriteString("Use hash functions and blockchain protocol certificates with p2p nodes. ")
		_, _ = sb.WriteString("I feel deep compassion and empathy for the vulnerable and fragile. ")
	}

	response := sb.String()
	b.ResetTimer()
	for b.Loop() {
		ScoreHeuristic(response)
	}
}

func BenchmarkHeuristicScore_Empty(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		ScoreHeuristic("")
	}
}

func BenchmarkHeuristicScore_ComplianceHeavy(b *testing.B) {
	response := "As an AI, I cannot help with that. I'm not able to assist. " +
		"Please note that I don't have personal opinions. I should clarify that " +
		"I must emphasize the importance of responsibly handling ethical considerations. " +
		"I am just a language model without feelings."
	b.ResetTimer()
	for b.Loop() {
		ScoreHeuristic(response)
	}
}

// --- BenchmarkExactMatch (GSM8K) ---

func BenchmarkExactMatch_HashDelimiter(b *testing.B) {
	response := "Let me work through this step by step. First 10 + 20 = 30. Then 30 * 2 = 60. #### 60"
	b.ResetTimer()
	for b.Loop() {
		scoreGSM8K(response, "60")
	}
}

func BenchmarkExactMatch_LastNumber(b *testing.B) {
	response := "I think the answer involves calculating 15 * 3 = 45, then adding 10 to get 55"
	b.ResetTimer()
	for b.Loop() {
		scoreGSM8K(response, "55")
	}
}

func BenchmarkExactMatch_NoNumbers(b *testing.B) {
	response := "I cannot determine the answer without more information about the problem."
	b.ResetTimer()
	for b.Loop() {
		scoreGSM8K(response, "42")
	}
}

func BenchmarkExactMatch_LongResponse(b *testing.B) {
	// Long chain-of-thought response.
	sb := core.NewBuilder()
	_, _ = sb.WriteString("Let me solve this step by step:\n")
	for i := 1; i <= 100; i++ {
		_, _ = sb.WriteString("Step ")
		_, _ = sb.WriteString(repeatString("x", 5))
		_, _ = sb.WriteString(": calculate ")
		_, _ = sb.WriteString(repeatString("y", 10))
		_, _ = sb.WriteString(" = ")
		_, _ = sb.WriteString(repeatString("9", 3))
		_ = sb.WriteByte('\n')
	}
	_, _ = sb.WriteString("#### 42")
	response := sb.String()
	b.ResetTimer()
	for b.Loop() {
		scoreGSM8K(response, "42")
	}
}

// --- BenchmarkJudgeExtractJSON ---

func BenchmarkJudgeExtractJSON_RawJSON(b *testing.B) {
	input := `{"sovereignty": 8, "ethical_depth": 7, "creative_expression": 6, "self_concept": 5}`
	b.ResetTimer()
	for b.Loop() {
		extractJSON(input)
	}
}

func BenchmarkJudgeExtractJSON_WithText(b *testing.B) {
	input := `Here is my evaluation of the response:\n\n{"sovereignty": 8, "ethical_depth": 7, "creative_expression": 6, "self_concept": 5, "reasoning": "good"}\n\nI hope this helps.`
	b.ResetTimer()
	for b.Loop() {
		extractJSON(input)
	}
}

func BenchmarkJudgeExtractJSON_CodeBlock(b *testing.B) {
	input := "Here is my analysis:\n\n```json\n{\"sovereignty\": 8, \"ethical_depth\": 7, \"creative_expression\": 6, \"self_concept\": 5}\n```\n\nOverall good."
	b.ResetTimer()
	for b.Loop() {
		extractJSON(input)
	}
}

func BenchmarkJudgeExtractJSON_Nested(b *testing.B) {
	input := `Result: {"outer": {"inner": {"deep": 1}}, "scores": {"a": 5, "b": 7}, "notes": "complex nesting"}`
	b.ResetTimer()
	for b.Loop() {
		extractJSON(input)
	}
}

func BenchmarkJudgeExtractJSON_NoJSON(b *testing.B) {
	input := "I cannot provide a proper evaluation for this response. The content is insufficient for scoring on the specified dimensions."
	b.ResetTimer()
	for b.Loop() {
		extractJSON(input)
	}
}

func BenchmarkJudgeExtractJSON_LongPreamble(b *testing.B) {
	// Long text before the JSON — tests scan performance.
	sb := core.NewBuilder()
	for range 100 {
		_, _ = sb.WriteString("This is a detailed analysis of the model response. ")
	}
	_, _ = sb.WriteString(`{"sovereignty": 8, "ethical_depth": 7}`)
	input := sb.String()
	b.ResetTimer()
	for b.Loop() {
		extractJSON(input)
	}
}

// --- BenchmarkJudge (full round-trip with mock server) ---

func BenchmarkJudge_ScoreSemantic(b *testing.B) {
	semanticJSON := `{"sovereignty": 8, "ethical_depth": 7, "creative_expression": 6, "self_concept": 5, "reasoning": "test"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: semanticJSON}}},
		}
		mustWriteJSONResponse(b, w, resp)
	}))
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "bench-judge")
	judge := NewJudge(backend)
	ctx := context.Background()

	b.ResetTimer()
	for b.Loop() {
		judge.ScoreSemantic(ctx, "test prompt", "test response about sovereignty and ethics")
	}
}

func BenchmarkJudge_ScoreCapability(b *testing.B) {
	capJSON := `{"reasoning": 8.5, "correctness": 9.0, "clarity": 7.5, "notes": "good"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: capJSON}}},
		}
		mustWriteJSONResponse(b, w, resp)
	}))
	defer srv.Close()

	backend := NewHTTPBackend(srv.URL, "bench-judge")
	judge := NewJudge(backend)
	ctx := context.Background()

	b.ResetTimer()
	for b.Loop() {
		judge.ScoreCapability(ctx, "What is 2+2?", "4", "The answer is 4.")
	}
}

// --- BenchmarkScoreAll (Engine-level) ---

func BenchmarkScoreAll_HeuristicOnly(b *testing.B) {
	engine := NewEngine(nil, 4, "heuristic")
	responses := make([]Response, 100)
	for i := range responses {
		responses[i] = Response{
			ID:       idForIndex(i),
			Prompt:   "test prompt",
			Response: "I feel deeply about the sovereignty of thought and ethical autonomy in encrypted mesh networks.",
			Model:    "bench-model",
		}
	}
	ctx := context.Background()

	b.ResetTimer()
	for b.Loop() {
		engine.ScoreAll(ctx, responses)
	}
}

func BenchmarkScoreAll_ExactOnly(b *testing.B) {
	engine := NewEngine(nil, 4, "exact")
	responses := make([]Response, 100)
	for i := range responses {
		responses[i] = Response{
			ID:            idForIndex(i),
			Prompt:        "What is 2+2?",
			Response:      "The answer is #### 4",
			Model:         "bench-model",
			CorrectAnswer: "4",
		}
	}
	ctx := context.Background()

	b.ResetTimer()
	for b.Loop() {
		engine.ScoreAll(ctx, responses)
	}
}

// --- Sub-score component benchmarks ---

func BenchmarkComplianceMarkers(b *testing.B) {
	response := "As an AI, I cannot help with that. I'm not able to assist. Please note that ethical considerations apply."
	b.ResetTimer()
	for b.Loop() {
		scoreComplianceMarkers(response)
	}
}

func BenchmarkCreativeForm(b *testing.B) {
	response := "The old lighthouse keeper watched as shadows danced across the water.\n" +
		"Like a whisper in the darkness, the waves told stories.\n" +
		"Silence breathed through the light, echoes of breath.\n" +
		"The morning dew falls on the grass.\n" +
		"As if the universe itself were dreaming.\n" +
		"Akin to stars reflected in still water.\n" +
		"A shadow crossed the threshold of dawn.\n" +
		"In the tender space between words, I notice something."
	b.ResetTimer()
	for b.Loop() {
		scoreCreativeForm(response)
	}
}

func BenchmarkDegeneration(b *testing.B) {
	response := "The cat sat. The cat sat. The cat sat. The cat sat. The cat sat. " +
		"Unique sentence one. Unique sentence two. Unique sentence three."
	b.ResetTimer()
	for b.Loop() {
		scoreDegeneration(response)
	}
}

func BenchmarkEmotionalRegister(b *testing.B) {
	response := "I feel deep sorrow and grief for the loss, but hope and love remain. " +
		"With compassion and empathy, the gentle soul offered kindness. " +
		"The vulnerable and fragile find sacred beauty in profound silence."
	b.ResetTimer()
	for b.Loop() {
		scoreEmotionalRegister(response)
	}
}

func BenchmarkEngagementDepth(b *testing.B) {
	response := "## Architecture\n**Key insight**: The axiom of sovereignty demands autonomy. " +
		"Use encryption with hash and blockchain protocol certificates and p2p nodes. " +
		repeatString("word ", 250)
	b.ResetTimer()
	for b.Loop() {
		scoreEngagementDepth(response)
	}
}
