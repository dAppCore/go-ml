package ml

import (
	"dappco.re/go"
	coreio "dappco.re/go/io"
)

func writeParquetJSONL(t *core.T, file string) {
	t.Helper()
	line := `{"messages":[{"role":"system","content":"sys"},{"role":"user","content":"prompt"},{"role":"assistant","content":"response"}]}` + "\n"
	core.RequireNoError(t, coreio.Local.Write(file, line))
}

func TestParquet_ExportParquet_Good(t *core.T) {
	dir := t.TempDir()
	writeParquetJSONL(t, core.JoinPath(dir, "train.jsonl"))
	count, err := ExportParquet(dir, "")
	core.RequireNoError(t, err)
	core.AssertEqual(t, 1, count)
	core.AssertTrue(t, coreio.Local.IsFile(core.JoinPath(dir, "parquet", "train.parquet")))
}

func TestParquet_ExportParquet_Bad(t *core.T) {
	count, err := ExportParquet(core.JoinPath(t.TempDir(), "missing"), "")
	core.AssertNoError(t, err)
	core.AssertEqual(t, 0, count)
}

func TestParquet_ExportParquet_Ugly(t *core.T) {
	dir := t.TempDir()
	core.RequireNoError(t, coreio.Local.Write(core.JoinPath(dir, "train.jsonl"), "not structured\n"))
	count, err := ExportParquet(dir, core.JoinPath(dir, "out"))
	core.RequireNoError(t, err)
	core.AssertEqual(t, 0, count)
}

func TestParquet_ExportSplitParquet_Good(t *core.T) {
	dir := t.TempDir()
	jsonl := core.JoinPath(dir, "split.jsonl")
	writeParquetJSONL(t, jsonl)
	count, err := ExportSplitParquet(jsonl, dir, "train")
	core.RequireNoError(t, err)
	core.AssertEqual(t, 1, count)
	core.AssertTrue(t, coreio.Local.IsFile(core.JoinPath(dir, "train.parquet")))
}

func TestParquet_ExportSplitParquet_Bad(t *core.T) {
	count, err := ExportSplitParquet(core.JoinPath(t.TempDir(), "missing.jsonl"), t.TempDir(), "bad")
	core.AssertError(t, err)
	core.AssertEqual(t, 0, count)
}

func TestParquet_ExportSplitParquet_Ugly(t *core.T) {
	file := core.JoinPath(t.TempDir(), "empty.jsonl")
	core.RequireNoError(t, coreio.Local.Write(file, "\n"))
	count, err := ExportSplitParquet(file, t.TempDir(), "empty")
	core.RequireNoError(t, err)
	core.AssertEqual(t, 0, count)
}
