// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	coreio "forge.lthn.ai/core/go-io"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// fakeTransport — in-memory RemoteTransport for testing
// ---------------------------------------------------------------------------

// fakeTransport implements RemoteTransport using canned responses keyed on
// a substring of the command string.  Commands are matched in insertion order
// so the first matching key wins.
type fakeTransport struct {
	commands []fakeCmd
}

type fakeCmd struct {
	pattern string
	stdout  string
	err     error
}

func newFakeTransport() *fakeTransport { return &fakeTransport{} }

func (f *fakeTransport) On(pattern, stdout string, err error) {
	f.commands = append(f.commands, fakeCmd{pattern: pattern, stdout: stdout, err: err})
}

func (f *fakeTransport) Run(_ context.Context, cmd string) (string, error) {
	for _, fc := range f.commands {
		if contains(cmd, fc.pattern) {
			return fc.stdout, fc.err
		}
	}
	return "", errors.New("fakeTransport: no match for command: " + cmd)
}

func (f *fakeTransport) CopyFrom(_ context.Context, _, _ string) error { return nil }
func (f *fakeTransport) CopyTo(_ context.Context, _, _ string) error   { return nil }

// contains is a small helper to avoid importing strings just for this.
func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && searchSubstr(s, substr)
}

func searchSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// =========================================================================
// 1. AdapterMeta tests
// =========================================================================

func TestAdapterMeta_KnownFamilies_Good(t *testing.T) {
	tests := []struct {
		dirname  string
		wantTag  string
		wantPfx  string
		wantStem string
	}{
		// gemma-3-1b via "1b" prefix
		{"adapters-1b", "gemma-3-1b", "G1", "1b"},
		// gemma-3-27b via "27b" prefix
		{"adapters-27b", "gemma-3-27b", "G27", "27b"},
		// deepseek-r1-7b
		{"adapters-deepseek-r1-7b", "deepseek-r1-7b", "R1", "deepseek-r1-7b"},
		// gpt-oss
		{"adapters-gpt-oss", "gpt-oss-20b", "GPT", "gpt-oss"},
		// gemma-3-12b via "12b" prefix
		{"adapters-12b", "gemma-3-12b", "G12", "12b"},
		// gemma-3-4b via "4b" prefix
		{"adapters-4b", "gemma-3-4b", "G4", "4b"},
		// bench-1b
		{"adapters-bench-1b", "gemma-3-1b", "B1", "bench-1b"},
		// book
		{"adapters-book", "gemma-3-27b", "Book", "book"},
		// cross
		{"adapters-cross", "gemma-3-12b", "Cross", "cross"},
		// vi → gemma-3-1b
		{"adapters-vi", "gemma-3-1b", "Vi1", "vi"},
		// vi-12b → gemma-3-12b
		{"adapters-vi-12b", "gemma-3-12b", "Vi12", "vi-12b"},
		// lem-gpt-oss
		{"adapters-lem-gpt-oss", "gpt-oss-20b", "LGPT", "lem-gpt-oss"},
	}

	for _, tt := range tests {
		t.Run(tt.dirname, func(t *testing.T) {
			tag, pfx, stem := AdapterMeta(tt.dirname)
			assert.Equal(t, tt.wantTag, tag, "model tag")
			assert.Equal(t, tt.wantPfx, pfx, "label prefix")
			assert.Equal(t, tt.wantStem, stem, "run ID stem")
		})
	}
}

func TestAdapterMeta_WithVariant_Good(t *testing.T) {
	// "adapters-27b-reasoning" → 27b prefix matches, variant = "reasoning"
	tag, pfx, stem := AdapterMeta("adapters-27b-reasoning")
	assert.Equal(t, "gemma-3-27b", tag)
	assert.Equal(t, "G27-reasoning", pfx)
	assert.Equal(t, "27b-reasoning", stem)
}

func TestAdapterMeta_WithoutVariant_Good(t *testing.T) {
	// "adapters-12b" → variant is empty → "base"
	tag, pfx, stem := AdapterMeta("adapters-12b")
	assert.Equal(t, "gemma-3-12b", tag)
	assert.Equal(t, "G12", pfx) // variant="base" produces short without suffix
	assert.Equal(t, "12b", stem)
}

func TestAdapterMeta_SubdirectoryPattern_Good(t *testing.T) {
	// "adapters-15k/gemma-3-27b" → matches "15k/gemma-3-27b" prefix
	tag, pfx, stem := AdapterMeta("adapters-15k/gemma-3-27b")
	assert.Equal(t, "gemma-3-27b", tag)
	assert.Equal(t, "G27", pfx)
	// stem should replace "/" with "-"
	assert.Equal(t, "15k-gemma-3-27b", stem)
}

func TestAdapterMeta_SubdirectoryWithVariant_Good(t *testing.T) {
	// "adapters-15k/gemma-3-1b-creative" → variant = "creative"
	tag, pfx, stem := AdapterMeta("adapters-15k/gemma-3-1b-creative")
	assert.Equal(t, "gemma-3-1b", tag)
	assert.Equal(t, "G1-creative", pfx)
	assert.Equal(t, "15k-gemma-3-1b-creative", stem)
}

func TestAdapterMeta_Unknown_Bad(t *testing.T) {
	// Unknown dirname falls back: tag=name, short=name[:10], stem=name
	tag, pfx, stem := AdapterMeta("adapters-completelynewmodel42")
	assert.Equal(t, "completelynewmodel42", tag)
	assert.Equal(t, "completely", pfx) // truncated to 10 chars
	assert.Equal(t, "completelynewmodel42", stem)
}

func TestAdapterMeta_UnknownShort_Good(t *testing.T) {
	// Short unknown name (< 10 chars) is not truncated.
	tag, pfx, stem := AdapterMeta("adapters-xyz")
	assert.Equal(t, "xyz", tag)
	assert.Equal(t, "xyz", pfx)
	assert.Equal(t, "xyz", stem)
}

func TestAdapterMeta_NoPrefix_Good(t *testing.T) {
	// dirname without "adapters-" prefix — TrimPrefix does nothing useful,
	// but the function should still handle it gracefully.
	tag, pfx, stem := AdapterMeta("27b-fancy")
	assert.Equal(t, "gemma-3-27b", tag)
	assert.Equal(t, "G27-fancy", pfx)
	assert.Equal(t, "27b-fancy", stem)
}

// =========================================================================
// 2. FindUnscored tests
// =========================================================================

func TestFindUnscored_AllUnscored_Good(t *testing.T) {
	checkpoints := []Checkpoint{
		{Dirname: "b-dir", Iteration: 200, RunID: "run-b", Label: "B @200"},
		{Dirname: "a-dir", Iteration: 100, RunID: "run-a", Label: "A @100"},
		{Dirname: "a-dir", Iteration: 50, RunID: "run-a", Label: "A @50"},
	}
	scored := map[[2]string]bool{}

	result := FindUnscored(checkpoints, scored)

	require.Len(t, result, 3)
	// Should be sorted by (dirname, iteration)
	assert.Equal(t, "a-dir", result[0].Dirname)
	assert.Equal(t, 50, result[0].Iteration)
	assert.Equal(t, "a-dir", result[1].Dirname)
	assert.Equal(t, 100, result[1].Iteration)
	assert.Equal(t, "b-dir", result[2].Dirname)
	assert.Equal(t, 200, result[2].Iteration)
}

func TestFindUnscored_SomeScored_Good(t *testing.T) {
	checkpoints := []Checkpoint{
		{Dirname: "dir", Iteration: 100, RunID: "run-1", Label: "L @100"},
		{Dirname: "dir", Iteration: 200, RunID: "run-1", Label: "L @200"},
		{Dirname: "dir", Iteration: 300, RunID: "run-1", Label: "L @300"},
	}
	scored := map[[2]string]bool{
		{"run-1", "L @100"}: true,
		{"run-1", "L @300"}: true,
	}

	result := FindUnscored(checkpoints, scored)

	require.Len(t, result, 1)
	assert.Equal(t, 200, result[0].Iteration)
	assert.Equal(t, "L @200", result[0].Label)
}

func TestFindUnscored_AllScored_Good(t *testing.T) {
	checkpoints := []Checkpoint{
		{Dirname: "dir", Iteration: 100, RunID: "run-1", Label: "L @100"},
		{Dirname: "dir", Iteration: 200, RunID: "run-1", Label: "L @200"},
	}
	scored := map[[2]string]bool{
		{"run-1", "L @100"}: true,
		{"run-1", "L @200"}: true,
	}

	result := FindUnscored(checkpoints, scored)
	assert.Empty(t, result)
}

func TestFindUnscored_EmptyInput_Good(t *testing.T) {
	result := FindUnscored(nil, nil)
	assert.Empty(t, result)

	result = FindUnscored([]Checkpoint{}, map[[2]string]bool{})
	assert.Empty(t, result)
}

func TestFindUnscored_NilScored_Good(t *testing.T) {
	// nil scored map should treat everything as unscored
	checkpoints := []Checkpoint{
		{Dirname: "a", Iteration: 1, RunID: "r", Label: "L @1"},
	}
	result := FindUnscored(checkpoints, nil)
	require.Len(t, result, 1)
}

// =========================================================================
// 3. BufferInfluxResult / ReplayInfluxBuffer round-trip tests
// =========================================================================

func TestBufferInfluxResult_RoundTrip_Good(t *testing.T) {
	workDir := t.TempDir()

	cp := Checkpoint{
		RemoteDir: "/data/adapters-27b",
		Filename:  "0001000_adapters.safetensors",
		Dirname:   "adapters-27b",
		Iteration: 1000,
		ModelTag:  "gemma-3-27b",
		Label:     "G27 @1000",
		RunID:     "27b-capability-auto",
	}
	results := ProbeResult{
		Accuracy: 75.0,
		Correct:  3,
		Total:    4,
		ByCategory: map[string]CategoryResult{
			"math": {Correct: 2, Total: 2},
			"lang": {Correct: 1, Total: 2},
		},
		Probes: map[string]SingleProbeResult{
			"p1": {Passed: true, Response: "ok"},
			"p2": {Passed: false, Response: "wrong"},
		},
	}

	BufferInfluxResult(workDir, cp, results)

	// Verify the buffer file exists and contains valid JSONL
	bufPath := filepath.Join(workDir, InfluxBufferFile)
	raw, err := coreio.Local.Read(bufPath)
	data := []byte(raw)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Parse the JSONL entry and verify fields
	var entry bufferEntry
	err = json.Unmarshal(data[:len(data)-1], &entry) // trim trailing newline
	require.NoError(t, err)
	assert.Equal(t, cp.Label, entry.Checkpoint.Label)
	assert.Equal(t, cp.ModelTag, entry.Checkpoint.ModelTag)
	assert.Equal(t, cp.RunID, entry.Checkpoint.RunID)
	assert.Equal(t, results.Accuracy, entry.Results.Accuracy)
	assert.Equal(t, results.Correct, entry.Results.Correct)
	assert.Equal(t, results.Total, entry.Results.Total)
	assert.NotEmpty(t, entry.Timestamp)
}

func TestBufferInfluxResult_MultipleEntries_Good(t *testing.T) {
	workDir := t.TempDir()

	for i := range 3 {
		cp := Checkpoint{
			Dirname:   "dir",
			Iteration: i * 100,
			Label:     "L",
			RunID:     "run",
			ModelTag:  "tag",
		}
		results := ProbeResult{
			Accuracy: float64(i) * 25.0,
			Correct:  i,
			Total:    4,
			Probes:   map[string]SingleProbeResult{},
		}
		BufferInfluxResult(workDir, cp, results)
	}

	bufPath := filepath.Join(workDir, InfluxBufferFile)
	raw, err := coreio.Local.Read(bufPath)
	data := []byte(raw)
	require.NoError(t, err)

	// Count newlines — should be 3 JSONL lines
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	assert.Equal(t, 3, lines)
}

func TestReplayInfluxBuffer_EmptyFile_Good(t *testing.T) {
	workDir := t.TempDir()

	// No buffer file exists — ReplayInfluxBuffer should be a no-op
	ReplayInfluxBuffer(workDir, nil)

	// Buffer file still shouldn't exist
	assert.False(t, coreio.Local.IsFile(filepath.Join(workDir, InfluxBufferFile)))
}

func TestReplayInfluxBuffer_MissingFile_Good(t *testing.T) {
	// Calling with a nonexistent directory should not panic
	ReplayInfluxBuffer("/nonexistent/path/that/does/not/exist", nil)
}

// =========================================================================
// 4. DiscoverCheckpoints tests (using fakeTransport)
// =========================================================================

func TestDiscoverCheckpoints_HappyPath_Good(t *testing.T) {
	ft := newFakeTransport()

	base := "/data/training"

	// Command 1: list adapter directories (exact command from DiscoverCheckpoints)
	ft.On("ls -d "+base+"/adapters-* 2>/dev/null",
		base+"/adapters-27b\n"+base+"/adapters-1b\n", nil)

	// Command 2a: sub-directory check for adapters-27b — no gemma-3-* subdirs
	ft.On("ls -d "+base+"/adapters-27b/gemma-3-* 2>/dev/null", "", errors.New("no match"))

	// Command 2b: sub-directory check for adapters-1b — no gemma-3-* subdirs
	ft.On("ls -d "+base+"/adapters-1b/gemma-3-* 2>/dev/null", "", errors.New("no match"))

	// Command 3a: list safetensors in adapters-27b
	ft.On("ls "+base+"/adapters-27b/*_adapters.safetensors 2>/dev/null",
		base+"/adapters-27b/0001000_adapters.safetensors\n"+base+"/adapters-27b/0002000_adapters.safetensors\n", nil)

	// Command 3b: list safetensors in adapters-1b
	ft.On("ls "+base+"/adapters-1b/*_adapters.safetensors 2>/dev/null",
		base+"/adapters-1b/0000500_adapters.safetensors\n", nil)

	cfg := &AgentConfig{
		M3AdapterBase: base,
		Transport:     ft,
	}

	checkpoints, err := DiscoverCheckpoints(cfg)
	require.NoError(t, err)
	require.Len(t, checkpoints, 3)

	// Verify parsed checkpoint details
	found1000 := false
	found2000 := false
	found500 := false
	for _, cp := range checkpoints {
		switch {
		case cp.Dirname == "adapters-27b" && cp.Iteration == 1000:
			found1000 = true
			assert.Equal(t, "gemma-3-27b", cp.ModelTag)
			assert.Equal(t, "0001000_adapters.safetensors", cp.Filename)
			assert.Contains(t, cp.Label, "@0001000")
			assert.Contains(t, cp.RunID, "27b")
		case cp.Dirname == "adapters-27b" && cp.Iteration == 2000:
			found2000 = true
		case cp.Dirname == "adapters-1b" && cp.Iteration == 500:
			found500 = true
			assert.Equal(t, "gemma-3-1b", cp.ModelTag)
		}
	}
	assert.True(t, found1000, "should find iteration 1000")
	assert.True(t, found2000, "should find iteration 2000")
	assert.True(t, found500, "should find iteration 500")
}

func TestDiscoverCheckpoints_WithSubDirs_Good(t *testing.T) {
	ft := newFakeTransport()

	base := "/data/training"

	// Command 1: list adapter directories
	ft.On("ls -d "+base+"/adapters-* 2>/dev/null",
		base+"/adapters-15k\n", nil)

	// Command 2: sub-directory check finds gemma-3-* subdirs
	ft.On("ls -d "+base+"/adapters-15k/gemma-3-* 2>/dev/null",
		base+"/adapters-15k/gemma-3-27b\n"+base+"/adapters-15k/gemma-3-1b\n", nil)

	// Command 3a: list safetensors in gemma-3-27b subdir
	ft.On("ls "+base+"/adapters-15k/gemma-3-27b/*_adapters.safetensors 2>/dev/null",
		base+"/adapters-15k/gemma-3-27b/0003000_adapters.safetensors\n", nil)

	// Command 3b: list safetensors in gemma-3-1b subdir
	ft.On("ls "+base+"/adapters-15k/gemma-3-1b/*_adapters.safetensors 2>/dev/null",
		base+"/adapters-15k/gemma-3-1b/0001500_adapters.safetensors\n", nil)

	cfg := &AgentConfig{
		M3AdapterBase: base,
		Transport:     ft,
	}

	checkpoints, err := DiscoverCheckpoints(cfg)
	require.NoError(t, err)
	require.Len(t, checkpoints, 2)

	// The dirname should include the subdirectory path relative to base
	for _, cp := range checkpoints {
		switch {
		case cp.Iteration == 3000:
			assert.Equal(t, "adapters-15k/gemma-3-27b", cp.Dirname)
			assert.Equal(t, "gemma-3-27b", cp.ModelTag)
		case cp.Iteration == 1500:
			assert.Equal(t, "adapters-15k/gemma-3-1b", cp.Dirname)
			assert.Equal(t, "gemma-3-1b", cp.ModelTag)
		default:
			t.Errorf("unexpected iteration %d", cp.Iteration)
		}
	}
}

func TestDiscoverCheckpoints_NoAdapters_Good(t *testing.T) {
	ft := newFakeTransport()
	base := "/data/training"

	// ls -d returns empty output
	ft.On("ls -d "+base+"/adapters-* 2>/dev/null", "", nil)

	cfg := &AgentConfig{
		M3AdapterBase: base,
		Transport:     ft,
	}

	checkpoints, err := DiscoverCheckpoints(cfg)
	require.NoError(t, err)
	assert.Empty(t, checkpoints)
}

func TestDiscoverCheckpoints_SSHError_Bad(t *testing.T) {
	ft := newFakeTransport()
	base := "/data/training"

	ft.On("ls -d "+base+"/adapters-* 2>/dev/null", "", errors.New("ssh: connection refused"))

	cfg := &AgentConfig{
		M3AdapterBase: base,
		Transport:     ft,
	}

	_, err := DiscoverCheckpoints(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list adapter dirs")
}

func TestDiscoverCheckpoints_FilterPattern_Good(t *testing.T) {
	ft := newFakeTransport()
	base := "/data/training"

	// When Filter is set, the ls pattern changes to adapters-27b*
	ft.On("ls -d "+base+"/adapters-27b* 2>/dev/null",
		base+"/adapters-27b\n", nil)

	// No gemma-3-* subdirs
	ft.On("ls -d "+base+"/adapters-27b/gemma-3-* 2>/dev/null", "", errors.New("no match"))

	ft.On("ls "+base+"/adapters-27b/*_adapters.safetensors 2>/dev/null",
		base+"/adapters-27b/0001000_adapters.safetensors\n", nil)

	cfg := &AgentConfig{
		M3AdapterBase: base,
		Transport:     ft,
		Filter:        "27b",
	}

	checkpoints, err := DiscoverCheckpoints(cfg)
	require.NoError(t, err)
	require.Len(t, checkpoints, 1)
	assert.Equal(t, 1000, checkpoints[0].Iteration)
}

func TestDiscoverCheckpoints_NoSafetensors_Good(t *testing.T) {
	ft := newFakeTransport()
	base := "/data/training"

	ft.On("ls -d "+base+"/adapters-* 2>/dev/null",
		base+"/adapters-27b\n", nil)
	ft.On("ls -d "+base+"/adapters-27b/gemma-3-* 2>/dev/null", "", errors.New("no match"))

	// safetensors listing fails (no checkpoint files yet)
	ft.On("ls "+base+"/adapters-27b/*_adapters.safetensors 2>/dev/null", "", errors.New("no match"))

	cfg := &AgentConfig{
		M3AdapterBase: base,
		Transport:     ft,
	}

	checkpoints, err := DiscoverCheckpoints(cfg)
	require.NoError(t, err)
	assert.Empty(t, checkpoints, "no safetensors means no checkpoints")
}
