package ml

import (
	"context"

	"dappco.re/go"
)

func TestExpand_GetCompletedIDs_Good(t *core.T) {
	influx, _ := newFakeInflux(t, map[string][]map[string]any{"expansion_gen": {{"seed_id": "s1"}}}, 0)
	ids, err := GetCompletedIDs(influx)
	core.RequireNoError(t, err)
	core.AssertTrue(t, ids["s1"])
}

func TestExpand_GetCompletedIDs_Bad(t *core.T) {
	influx := &InfluxClient{url: "http://127.0.0.1:1", db: "test"}
	ids, err := GetCompletedIDs(influx)
	core.AssertError(t, err)
	core.AssertNil(t, ids)
}

func TestExpand_GetCompletedIDs_Ugly(t *core.T) {
	influx, _ := newFakeInflux(t, map[string][]map[string]any{"expansion_gen": {{"seed_id": ""}}}, 0)
	ids, err := GetCompletedIDs(influx)
	core.RequireNoError(t, err)
	core.AssertEmpty(t, ids)
}

func TestExpand_ExpandPrompts_Good(t *core.T) {
	influx, _ := newFakeInflux(t, map[string][]map[string]any{"expansion_gen": {}}, 0)
	prompts := []Response{{ID: "s1", Domain: "ethics", Prompt: "prompt"}}
	err := ExpandPrompts(context.Background(), &testBackend{result: Result{Text: "generated response"}}, influx, prompts, "m", "w", t.TempDir(), true, 1)
	core.RequireNoError(t, err)
	core.AssertLen(t, prompts, 1)
}

func TestExpand_ExpandPrompts_Bad(t *core.T) {
	influx, _ := newFakeInflux(t, map[string][]map[string]any{"expansion_gen": {{"seed_id": "s1"}}}, 0)
	prompts := []Response{{ID: "s1", Domain: "ethics", Prompt: "prompt"}}
	err := ExpandPrompts(context.Background(), &testBackend{}, influx, prompts, "m", "w", t.TempDir(), false, 0)
	core.RequireNoError(t, err)
	core.AssertLen(t, prompts, 1)
}

func TestExpand_ExpandPrompts_Ugly(t *core.T) {
	influx, _ := newFakeInflux(t, map[string][]map[string]any{"expansion_gen": {}}, 0)
	prompts := []Response{{ID: "s1", Domain: "ethics", Prompt: "prompt"}}
	err := ExpandPrompts(context.Background(), &testBackend{err: core.AnError}, influx, prompts, "m", "w", t.TempDir(), false, 0)
	core.RequireNoError(t, err)
	core.AssertLen(t, prompts, 1)
}
