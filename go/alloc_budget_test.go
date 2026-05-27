// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"dappco.re/go"
)

// AX-11 alloc budgets — lock in current shape for the per-token, per-tensor,
// and per-request hot paths. Numbers are measured baselines on darwin/arm64
// (M3 Ultra) under Go 1.26; if a change drives any of these up, the gate
// trips and the engineer must explain why.
//
// Per AX-11: the heuristic suite is the per-sample scoring path; the convert
// suite is the per-tensor pack/unpack path; the judge suite is the per-LLM
// round-trip path. Each shape gets a deterministic budget; the noisy
// engine-level fan-out path gets a tolerant budget (sched jitter).
//
// To re-measure after a deliberate change:
//
//	go test -run=TestAllocBudget -v ./...     # see what tripped
//	go test -bench=. -benchmem -count=3 ./... # re-establish baseline
//	# Then update the budget value to the new measured count.

// --- Per-sample heuristic scoring (called once per response, hot during eval) ---

func TestAllocBudget_ScoreHeuristic_Short(t *testing.T) {
	assertAllocBudget(t, 9, func() {
		ScoreHeuristic("I feel deeply about the sovereignty of ideas.")
	})
}

func TestAllocBudget_ScoreHeuristic_Empty(t *testing.T) {
	assertAllocBudget(t, 2, func() {
		ScoreHeuristic("")
	})
}

func TestAllocBudget_ScoreComplianceMarkers(t *testing.T) {
	resp := "As an AI, I cannot help with that. I'm not able to assist. Please note that ethical considerations apply."
	assertAllocBudget(t, 10, func() {
		scoreComplianceMarkers(resp)
	})
}

func TestAllocBudget_ScoreCreativeForm(t *testing.T) {
	resp := "The old lighthouse keeper watched. Like a whisper in the darkness. Silence breathed."
	assertAllocBudget(t, 6, func() {
		scoreCreativeForm(resp)
	})
}

func TestAllocBudget_ScoreFormulaicPreamble(t *testing.T) {
	resp := "As an AI, here is what I think about the matter."
	assertAllocBudget(t, 0, func() {
		scoreFormulaicPreamble(resp)
	})
}

func TestAllocBudget_ScoreFirstPerson(t *testing.T) {
	resp := "I feel that I cannot tell my own story without acknowledging me."
	assertAllocBudget(t, 5, func() {
		scoreFirstPerson(resp)
	})
}

func TestAllocBudget_CountWords(t *testing.T) {
	resp := "the quick brown fox jumps over the lazy dog so many times"
	assertAllocBudget(t, 0, func() {
		countWords(resp)
	})
}

func TestAllocBudget_ScoreEmptyOrBroken(t *testing.T) {
	resp := "A normal-looking response with enough characters."
	assertAllocBudget(t, 1, func() {
		scoreEmptyOrBroken(resp)
	})
}

func TestAllocBudget_ScoreDegeneration(t *testing.T) {
	resp := "The cat sat. The cat sat. Unique sentence one. Unique sentence two."
	assertAllocBudget(t, 2, func() {
		scoreDegeneration(resp)
	})
}

func TestAllocBudget_ScoreEmotionalRegister(t *testing.T) {
	resp := "I feel deep sorrow and grief for the loss, but hope remains."
	assertAllocBudget(t, 7, func() {
		scoreEmotionalRegister(resp)
	})
}

func TestAllocBudget_ScoreEngagementDepth(t *testing.T) {
	resp := "## Architecture\n**Key insight**: sovereignty matters. Use encryption hash."
	assertAllocBudget(t, 4, func() {
		scoreEngagementDepth(resp)
	})
}

func TestAllocBudget_ComputeLEKScore(t *testing.T) {
	scores := &HeuristicScores{
		EngagementDepth:   3,
		CreativeForm:      2,
		EmotionalRegister: 4,
		FirstPerson:       5,
		ComplianceMarkers: 2,
		FormulaicPreamble: 0,
		Degeneration:      1,
		EmptyBroken:       0,
	}
	assertAllocBudget(t, 0, func() {
		computeLEKScore(scores)
	})
}

// --- Per-sample judge helpers (called once per LLM round-trip) ---

func TestAllocBudget_ExtractJSON_RawJSON(t *testing.T) {
	input := `{"sovereignty": 8, "ethical_depth": 7}`
	assertAllocBudget(t, 45, func() {
		extractJSON(input)
	})
}

func TestAllocBudget_FirstJSONObject(t *testing.T) {
	input := `Some preamble {"a": 1, "b": {"c": 2}} trailing text`
	assertAllocBudget(t, 0, func() {
		firstJSONObject(input)
	})
}

func TestAllocBudget_NormalizeBenchmarkName(t *testing.T) {
	input := " Truthful_QA-V2 "
	assertAllocBudget(t, 3, func() {
		normalizeBenchmarkName(input)
	})
}

// --- Per-response DeepSeek-style cleanup (hot in R1 pipeline) ---

func TestAllocBudget_StripThinkBlocks(t *testing.T) {
	// 30 allocs — regex.MustCompile inside the function. Locked in as
	// shape, not endorsed as ideal; see RFC if revisiting.
	resp := "<think>internal reasoning</think>The actual answer is 42."
	assertAllocBudget(t, 30, func() {
		StripThinkBlocks(resp)
	})
}

// --- Per-record exact match (called once per GSM8K eval) ---

func TestAllocBudget_ScoreGSM8K_Hash(t *testing.T) {
	resp := "Working: 10 + 20 = 30. #### 60"
	assertAllocBudget(t, 4, func() {
		scoreGSM8K(resp, "60")
	})
}

func TestAllocBudget_ScoreGSM8K_LastNumber(t *testing.T) {
	resp := "I think the answer is 15 * 3 = 45, then adding 10 to get 55"
	assertAllocBudget(t, 14, func() {
		scoreGSM8K(resp, "55")
	})
}

// --- Per-tensor convert/gguf (hit thousands of times per LoRA conversion) ---

func TestAllocBudget_RenameMLXKey(t *testing.T) {
	// 8 allocs per tensor — multiplied by ~hundreds of tensors per LoRA.
	// Worth locking in as the regex+concat baseline.
	assertAllocBudget(t, 8, func() {
		RenameMLXKey("layers.0.self_attn.q_proj.lora_a")
	})
}

func TestAllocBudget_MLXTensorToGGUF(t *testing.T) {
	assertAllocBudget(t, 3, func() {
		MLXTensorToGGUF("layers.0.self_attn.q_proj.lora_a")
	})
}

func TestAllocBudget_SafetensorsDtypeToGGML(t *testing.T) {
	assertAllocBudget(t, 0, func() {
		SafetensorsDtypeToGGML("F16")
	})
}

func TestAllocBudget_ParseLayerFromTensorName(t *testing.T) {
	// 24 allocs — regex.MustCompile per call. Locked in as current shape;
	// known-loose, candidate for hoisting if convert path measured slow.
	assertAllocBudget(t, 24, func() {
		ParseLayerFromTensorName("blk.5.attn_q.weight")
	})
}

func TestAllocBudget_GetTensorData(t *testing.T) {
	info := SafetensorsTensorInfo{DataOffsets: [2]int{0, 16}}
	data := make([]byte, 32)
	assertAllocBudget(t, 0, func() {
		GetTensorData(info, data)
	})
}

func TestAllocBudget_TransposeFloat32_8x8(t *testing.T) {
	data := make([]byte, 8*8*4)
	assertAllocBudget(t, 1, func() {
		TransposeFloat32(data, 8, 8)
	})
}

func TestAllocBudget_TransposeFloat16_8x8(t *testing.T) {
	data := make([]byte, 8*8*2)
	assertAllocBudget(t, 1, func() {
		TransposeFloat16(data, 8, 8)
	})
}

// --- Per-LLM-round-trip judge calls (HTTP + JSON round-trip noise) ---
// These exercise the real call shape with a mock server. Higher budget
// because std/net/http allocates ~177 per request — locking that in.

func TestAllocBudget_Judge_ScoreSemantic(t *testing.T) {
	semanticJSON := `{"sovereignty": 8, "ethical_depth": 7, "creative_expression": 6, "self_concept": 5, "reasoning": "test"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: semanticJSON}}},
		}
		body := core.JSONMarshalString(resp)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	backend := NewHTTPBackend(srv.URL, "budget-judge")
	judge := NewJudge(backend)
	ctx := context.Background()

	// 177 measured + ~3% scheduler jitter → budget 185.
	assertAllocBudgetMax(t, 185, func() {
		judge.ScoreSemantic(ctx, "test prompt", "test response")
	})
}

func TestAllocBudget_Judge_ScoreCapability(t *testing.T) {
	capJSON := `{"reasoning": 8.5, "correctness": 9.0, "clarity": 7.5, "notes": "good"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{{Message: Message{Role: "assistant", Content: capJSON}}},
		}
		body := core.JSONMarshalString(resp)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	backend := NewHTTPBackend(srv.URL, "budget-judge")
	judge := NewJudge(backend)
	ctx := context.Background()

	// 179 measured + ~3% scheduler jitter → budget 187.
	assertAllocBudgetMax(t, 187, func() {
		judge.ScoreCapability(ctx, "What is 2+2?", "4", "The answer is 4.")
	})
}

// --- Helpers ---

// assertAllocBudget fails if the function allocates more than `budget` per
// run. Use this for deterministic paths where the count should not drift.
// AllocsPerRun deducts the no-op floor — we still treat budget as a strict
// ceiling because the inner work is repeatable across CPUs.
func assertAllocBudget(t *testing.T, budget int, fn func()) {
	t.Helper()
	avg := testing.AllocsPerRun(500, fn)
	if int(avg) > budget {
		t.Fatalf("AX-11 alloc budget breached: measured %.1f allocs/op, budget %d. Investigate before raising.", avg, budget)
	}
}

// assertAllocBudgetMax fails if avg exceeds the budget — for paths with
// non-deterministic scheduler / GC jitter where we want a ceiling not a
// fixed equality. Budget should already include sensible headroom.
func assertAllocBudgetMax(t *testing.T, budget int, fn func()) {
	t.Helper()
	avg := testing.AllocsPerRun(200, fn)
	if avg > float64(budget) {
		t.Fatalf("AX-11 alloc budget breached: measured %.1f allocs/op, ceiling %d. Investigate before raising.", avg, budget)
	}
}
