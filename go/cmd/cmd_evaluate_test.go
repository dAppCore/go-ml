// SPDX-License-Identifier: EUPL-1.2

package cmd

import (
	"testing"

	"dappco.re/go"
	"dappco.re/go/ml"
)

// TestEvaluate_decodeResponsesJSONL_Good parses a well-formed JSONL blob.
//
//	decodeResponsesJSONL(`{"id":"a","prompt":"p","response":"r","model":"m"}`)
func TestEvaluate_decodeResponsesJSONL_Good(t *testing.T) {
	input := `{"id":"a","prompt":"p1","response":"r1","model":"m1"}
{"id":"b","prompt":"p2","response":"r2","model":"m1"}
# comment line — skipped

{"id":"c","prompt":"p3","response":"r3","model":"m2"}
`
	r := decodeResponsesJSONL(input)
	if !r.OK {
		t.Fatalf("decodeResponsesJSONL err = %v", r.Value.(error))
	}
	got := r.Value.([]ml.Response)
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	if got[0].ID != "a" || got[2].ID != "c" {
		t.Errorf("ids = [%q … %q], want [a … c]", got[0].ID, got[2].ID)
	}
}

// TestEvaluate_decodeResponsesJSONL_Bad surfaces a clean error for malformed JSON.
//
//	decodeResponsesJSONL("{not json}")  // → Fail
func TestEvaluate_decodeResponsesJSONL_Bad(t *testing.T) {
	if r := decodeResponsesJSONL("{not json}\n"); r.OK {
		t.Error("expected result error for malformed JSON")
	}
}

// TestEvaluate_decodeResponsesJSONL_Ugly handles odd-but-tolerable input.
//
//	decodeResponsesJSONL("")                    // → Ok(nil)
//	decodeResponsesJSONL("\n\n# header\n\n")    // → Ok(nil)
func TestEvaluate_decodeResponsesJSONL_Ugly(t *testing.T) {
	// Empty string.
	r := decodeResponsesJSONL("")
	if !r.OK {
		t.Errorf("empty: err=%v", r.Value.(error))
	} else if out, _ := r.Value.([]ml.Response); len(out) != 0 {
		t.Errorf("empty: out=%v", out)
	}

	// Comments and blank lines only.
	r = decodeResponsesJSONL("\n\n# only comments\n\n")
	if !r.OK {
		t.Errorf("comments only: err=%v", r.Value.(error))
	} else if out, _ := r.Value.([]ml.Response); len(out) != 0 {
		t.Errorf("comments only: out=%v", out)
	}
}

// TestEvaluate_truncateLine_Good ensures error messages stay bounded.
//
//	truncateLine("abcdefgh", 4)  // "abcd…"
func TestEvaluate_truncateLine_Good(t *testing.T) {
	if got := truncateLine("abcdefgh", 4); got != "abcd…" {
		t.Errorf("truncateLine = %q, want %q", got, "abcd…")
	}
	if got := truncateLine("short", 10); got != "short" {
		t.Errorf("truncateLine under limit = %q, want %q", got, "short")
	}
}

// TestEvaluate_addEvaluateCommand_Good registers under the ml/evaluate path.
//
//	core ml evaluate --input file.jsonl
func TestEvaluate_addEvaluateCommand_Good(t *testing.T) {
	c := core.New()
	addEvaluateCommand(c)

	// The command must be reachable via the canonical path. Core returns a
	// Result whose Value holds the Command struct; we only care that the
	// path resolves with OK=true.
	if res := c.Command("ml/evaluate"); !res.OK {
		t.Fatal("ml/evaluate not registered")
	}
}
