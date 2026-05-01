package ml

import (
	"dappco.re/go"
	"dappco.re/go/store"
)

func seedCoverageDB(t *core.T) *store.DuckDB {
	t.Helper()
	db := newStoreDuckDB(t)
	core.RequireNoError(t, db.Exec(`CREATE TABLE seeds (
		source_file VARCHAR, region VARCHAR, seed_id VARCHAR, domain VARCHAR, prompt VARCHAR
	)`))
	return db
}

func TestCoverage_PrintCoverage_Good(t *core.T) {
	db := seedCoverageDB(t)
	core.RequireNoError(t, db.Exec("INSERT INTO seeds VALUES ('f','en-us','s1','ethics','prompt')"))
	buf := core.NewBuffer(nil)
	err := PrintCoverage(db, buf)
	core.RequireNoError(t, err)
	core.AssertContains(t, buf.String(), "Total seeds: 1")
}

func TestCoverage_PrintCoverage_Bad(t *core.T) {
	db := newStoreDuckDB(t)
	err := PrintCoverage(db, core.NewBuffer(nil))
	core.AssertError(t, err)
}

func TestCoverage_PrintCoverage_Ugly(t *core.T) {
	db := seedCoverageDB(t)
	buf := core.NewBuffer(nil)
	err := PrintCoverage(db, buf)
	core.RequireNoError(t, err)
	core.AssertContains(t, buf.String(), "Total seeds: 0")
}
