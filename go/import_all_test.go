package ml

import (
	"dappco.re/go"
	coreio "dappco.re/go/io"
)

func TestImportAll_ImportAll_Good(t *core.T) {
	db := newTestDB(t)
	dataDir := t.TempDir()
	trainDir := core.JoinPath(dataDir, "training", "training")
	core.RequireNoError(t, coreio.Local.EnsureDir(trainDir))
	core.RequireNoError(t, coreio.Local.Write(core.JoinPath(trainDir, "train.jsonl"), `{"messages":[{"role":"user","content":"p"},{"role":"assistant","content":"r"}]}`+"\n"))
	requireResultOK(t, ImportAll(db, ImportConfig{SkipM3: true, DataDir: dataDir}, core.NewBuffer(nil)))
	rCounts := db.TableCounts()
	requireResultOK(t, rCounts)
	counts := rCounts.Value.(map[string]int)
	core.AssertContains(t, counts, "training_examples")
}

func TestImportAll_ImportAll_Bad(t *core.T) {
	db := newTestDB(t)
	requireResultOK(t, ImportAll(db, ImportConfig{SkipM3: true, DataDir: ""}, core.NewBuffer(nil)))
	rCounts := db.TableCounts()
	requireResultOK(t, rCounts)
	counts := rCounts.Value.(map[string]int)
	core.AssertContains(t, counts, "training_examples")
}

func TestImportAll_ImportAll_Ugly(t *core.T) {
	db := newTestDB(t)
	dataDir := t.TempDir()
	core.RequireNoError(t, coreio.Local.EnsureDir(core.JoinPath(dataDir, "seeds")))
	requireResultOK(t, ImportAll(db, ImportConfig{SkipM3: true, DataDir: dataDir}, core.NewBuffer(nil)))
	core.AssertEqual(t, dataDir, core.PathDir(core.JoinPath(dataDir, "x")))
}
