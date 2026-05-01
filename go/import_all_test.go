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
	err := ImportAll(db, ImportConfig{SkipM3: true, DataDir: dataDir}, core.NewBuffer(nil))
	core.RequireNoError(t, err)
	counts, countErr := db.TableCounts()
	core.RequireNoError(t, countErr)
	core.AssertContains(t, counts, "training_examples")
}

func TestImportAll_ImportAll_Bad(t *core.T) {
	db := newTestDB(t)
	err := ImportAll(db, ImportConfig{SkipM3: true, DataDir: ""}, core.NewBuffer(nil))
	core.RequireNoError(t, err)
	counts, countErr := db.TableCounts()
	core.RequireNoError(t, countErr)
	core.AssertContains(t, counts, "training_examples")
}

func TestImportAll_ImportAll_Ugly(t *core.T) {
	db := newTestDB(t)
	dataDir := t.TempDir()
	core.RequireNoError(t, coreio.Local.EnsureDir(core.JoinPath(dataDir, "seeds")))
	err := ImportAll(db, ImportConfig{SkipM3: true, DataDir: dataDir}, core.NewBuffer(nil))
	core.RequireNoError(t, err)
	core.AssertEqual(t, dataDir, core.PathDir(core.JoinPath(dataDir, "x")))
}
