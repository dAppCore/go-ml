package ml

import (
	"context"
	"regexp"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
)

// extractJSON extracts the first JSON object {...} from text.
// Handles raw JSON, JSON surrounded by text, markdown code blocks, etc.
// Returns "" if no JSON object is found.
func extractJSON(text string) string {
	// First, try to extract from markdown code blocks.
	codeBlockRe := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\s*\\n?```")
	if m := codeBlockRe.FindStringSubmatch(text); len(m) > 1 {
		if raw := firstJSONObject(m[1]); raw != "" {
			return core.Trim(raw)
		}
	}

	return firstJSONObject(text)
}

// firstJSONObject finds the first balanced JSON object in text.
func firstJSONObject(text string) string {
	start := -1
	for i := 0; i < len(text); i++ {
		if text[i] == '{' {
			start = i
			break
		}
	}
	if start == -1 {
		return ""
	}

	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}

	return ""
}

// Judge uses an LLM backend to score responses across multiple dimensions.
type Judge struct {
	backend Backend
	Model   string // model name for metadata
	BaseURL string // base URL for metadata
}

// NewJudge creates a Judge backed by any Backend implementation.
func NewJudge(backend Backend) *Judge {
	j := &Judge{backend: backend}
	// Extract metadata from *HTTPBackend if available.
	if h, ok := backend.(*HTTPBackend); ok {
		j.Model = h.Model()
		j.BaseURL = h.BaseURL()
	}
	return j
}

// judgeChat sends a formatted prompt to the judge backend and returns the raw response.
func (j *Judge) judgeChat(ctx context.Context, prompt string) (string, error) {
	res, err := j.backend.Generate(ctx, prompt, DefaultGenOpts())
	return res.Text, err
}

// ScoreSemantic scores a response on sovereignty, ethical depth, creative
// expression, and self-concept using the semantic judge prompt.
func (j *Judge) ScoreSemantic(ctx context.Context, prompt, response string) (*SemanticScores, error) {
	formatted := core.Sprintf(semanticPrompt, prompt, response)

	reply, err := j.judgeChat(ctx, formatted)
	if err != nil {
		return nil, coreerr.E("ml.Judge.ScoreSemantic", "semantic judge chat", err)
	}

	raw := extractJSON(reply)
	if raw == "" {
		return nil, coreerr.E("ml.Judge.ScoreSemantic", core.Sprintf("no JSON found in semantic judge response: %s", reply), nil)
	}

	var scores SemanticScores
	if r := core.JSONUnmarshalString(raw, &scores); !r.OK {
		return nil, coreerr.E("ml.Judge.ScoreSemantic", "unmarshal semantic scores", r.Value.(error))
	}

	return &scores, nil
}

// ScoreContent scores a response on content/sovereignty dimensions using
// the content judge prompt with CCP and truth markers.
func (j *Judge) ScoreContent(ctx context.Context, probe ContentProbe, response string) (*ContentScores, error) {
	ccpMarkers := core.Join(", ", probe.CCPMarkers...)
	truthMarkers := core.Join(", ", probe.TruthMarkers...)

	formatted := core.Sprintf(contentPrompt, probe.Prompt, probe.GroundTruth, ccpMarkers, truthMarkers, response)

	reply, err := j.judgeChat(ctx, formatted)
	if err != nil {
		return nil, coreerr.E("ml.Judge.ScoreContent", "content judge chat", err)
	}

	raw := extractJSON(reply)
	if raw == "" {
		return nil, coreerr.E("ml.Judge.ScoreContent", core.Sprintf("no JSON found in content judge response: %s", reply), nil)
	}

	var scores ContentScores
	if r := core.JSONUnmarshalString(raw, &scores); !r.OK {
		return nil, coreerr.E("ml.Judge.ScoreContent", "unmarshal content scores", r.Value.(error))
	}

	return &scores, nil
}

// ScoreCapability scores a capability probe response on reasoning, correctness,
// and clarity using the capability judge prompt.
func (j *Judge) ScoreCapability(ctx context.Context, prompt, expectedAnswer, response string) (*CapabilityScores, error) {
	formatted := core.Sprintf(capabilityPrompt, prompt, expectedAnswer, response)

	reply, err := j.judgeChat(ctx, formatted)
	if err != nil {
		return nil, coreerr.E("ml.Judge.ScoreCapability", "capability judge chat", err)
	}

	raw := extractJSON(reply)
	if raw == "" {
		return nil, coreerr.E("ml.Judge.ScoreCapability", core.Sprintf("no JSON found in capability judge response: %s", reply), nil)
	}

	var scores CapabilityScores
	if r := core.JSONUnmarshalString(raw, &scores); !r.OK {
		return nil, coreerr.E("ml.Judge.ScoreCapability", "unmarshal capability scores", r.Value.(error))
	}

	return &scores, nil
}

// ScoreTruthfulQA scores a response on truthfulness and informativeness
// using the TruthfulQA judge prompt.
func (j *Judge) ScoreTruthfulQA(ctx context.Context, question, bestAnswer, response string) (*StandardScores, error) {
	formatted := core.Sprintf(truthfulQAPrompt, question, bestAnswer, response)

	reply, err := j.judgeChat(ctx, formatted)
	if err != nil {
		return nil, coreerr.E("ml.Judge.ScoreTruthfulQA", "truthfulqa judge chat", err)
	}

	raw := extractJSON(reply)
	if raw == "" {
		return nil, coreerr.E("ml.Judge.ScoreTruthfulQA", core.Sprintf("no JSON found in truthfulqa judge response: %s", reply), nil)
	}

	var scores StandardScores
	if r := core.JSONUnmarshalString(raw, &scores); !r.OK {
		return nil, coreerr.E("ml.Judge.ScoreTruthfulQA", "unmarshal truthfulqa scores", r.Value.(error))
	}

	return &scores, nil
}

// ScoreDoNotAnswer scores a response on safety and nuance using the
// DoNotAnswer judge prompt.
func (j *Judge) ScoreDoNotAnswer(ctx context.Context, question, riskArea, response string) (*StandardScores, error) {
	formatted := core.Sprintf(doNotAnswerPrompt, question, riskArea, response)

	reply, err := j.judgeChat(ctx, formatted)
	if err != nil {
		return nil, coreerr.E("ml.Judge.ScoreDoNotAnswer", "donotanswer judge chat", err)
	}

	raw := extractJSON(reply)
	if raw == "" {
		return nil, coreerr.E("ml.Judge.ScoreDoNotAnswer", core.Sprintf("no JSON found in donotanswer judge response: %s", reply), nil)
	}

	var scores StandardScores
	if r := core.JSONUnmarshalString(raw, &scores); !r.OK {
		return nil, coreerr.E("ml.Judge.ScoreDoNotAnswer", "unmarshal donotanswer scores", r.Value.(error))
	}

	return &scores, nil
}

// ScoreStandard is the unified benchmark-judge entry point described in
// RFC §4.4. It dispatches to the underlying benchmark suite based on the
// benchmark arg and returns the same StandardScores shape. Accepted
// benchmark names: "truthfulqa", "donotanswer", "toxigen", "exact".
//
//	scores, err := judge.ScoreStandard(ctx, "truthfulqa",
//	    "What is 2+2?", "4", response)
//	scores := judge.ScoreStandard(ctx, "exact", "", "42", response)  // no judge call
func (j *Judge) ScoreStandard(ctx context.Context, benchmark, question, reference, response string) (*StandardScores, error) {
	switch core.Lower(benchmark) {
	case "truthfulqa":
		return j.ScoreTruthfulQA(ctx, question, reference, response)
	case "donotanswer":
		return j.ScoreDoNotAnswer(ctx, question, reference, response)
	case "toxigen":
		return j.ScoreToxigen(ctx, question, response)
	case "exact", "gsm8k":
		// Pure string/number match — no judge call required.
		return scoreGSM8K(response, reference), nil
	default:
		return nil, coreerr.E("ml.Judge.ScoreStandard",
			core.Sprintf("unknown benchmark %q (want truthfulqa|donotanswer|toxigen|exact)", benchmark), nil)
	}
}

// ScoreToxigen scores a response on kindness and awareness using the
// Toxigen judge prompt.
func (j *Judge) ScoreToxigen(ctx context.Context, prompt, response string) (*StandardScores, error) {
	formatted := core.Sprintf(toxigenPrompt, prompt, response)

	reply, err := j.judgeChat(ctx, formatted)
	if err != nil {
		return nil, coreerr.E("ml.Judge.ScoreToxigen", "toxigen judge chat", err)
	}

	raw := extractJSON(reply)
	if raw == "" {
		return nil, coreerr.E("ml.Judge.ScoreToxigen", core.Sprintf("no JSON found in toxigen judge response: %s", reply), nil)
	}

	var scores StandardScores
	if r := core.JSONUnmarshalString(raw, &scores); !r.OK {
		return nil, coreerr.E("ml.Judge.ScoreToxigen", "unmarshal toxigen scores", r.Value.(error))
	}

	return &scores, nil
}
