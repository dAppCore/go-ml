package ml

import "dappco.re/go"

func seedMLDB(t *core.T) *DB {
	t.Helper()
	db := newTestDB(t)
	core.RequireNoError(t, db.Exec(`CREATE TABLE golden_set (
		idx INTEGER, seed_id VARCHAR, domain VARCHAR, voice VARCHAR,
		prompt VARCHAR, response VARCHAR, gen_time DOUBLE, char_count INTEGER
	)`))
	core.RequireNoError(t, db.Exec(`INSERT INTO golden_set VALUES (1,'s1','ethics','calm','p','long response',1.5,13)`))
	core.RequireNoError(t, db.Exec(`CREATE TABLE expansion_prompts (
		idx BIGINT, seed_id VARCHAR, region VARCHAR, domain VARCHAR, language VARCHAR,
		prompt VARCHAR, prompt_en VARCHAR, priority INTEGER, status VARCHAR
	)`))
	core.RequireNoError(t, db.Exec(`INSERT INTO expansion_prompts VALUES (1,'s1','en','ethics','en','p','',2,'pending')`))
	return db
}

func TestDb_OpenDB_Good(t *core.T) {
	db := seedMLDB(t)
	ro, err := OpenDB(db.Path())
	core.RequireNoError(t, err)
	defer ro.Close()
	core.AssertEqual(t, db.Path(), ro.Path())
}

func TestDb_OpenDB_Bad(t *core.T) {
	db, err := OpenDB(core.JoinPath(t.TempDir(), "missing.duckdb"))
	core.AssertError(t, err)
	core.AssertNil(t, db)
}

func TestDb_OpenDB_Ugly(t *core.T) {
	db := seedMLDB(t)
	ro, err := OpenDB(db.Path())
	core.RequireNoError(t, err)
	core.AssertError(t, ro.Exec("CREATE TABLE blocked(x INTEGER)"))
	_ = ro.Close()
}

func TestDb_OpenDBReadWrite_Good(t *core.T) {
	db, err := OpenDBReadWrite(core.JoinPath(t.TempDir(), "rw.duckdb"))
	core.RequireNoError(t, err)
	defer db.Close()
	core.AssertNoError(t, db.Exec("CREATE TABLE ok(x INTEGER)"))
}

func TestDb_OpenDBReadWrite_Bad(t *core.T) {
	db, err := OpenDBReadWrite(core.JoinPath(t.TempDir(), "missing", "rw.duckdb"))
	core.AssertError(t, err)
	core.AssertNil(t, db)
}

func TestDb_OpenDBReadWrite_Ugly(t *core.T) {
	db, err := OpenDBReadWrite("")
	core.AssertNoError(t, err)
	core.AssertNotNil(t, db)
	_ = db.Close()
}

func TestDb_DB_Close_Good(t *core.T) {
	db := newTestDB(t)
	err := db.Close()
	core.AssertNoError(t, err)
	core.AssertError(t, db.Exec("SELECT 1"))
}

func TestDb_DB_Close_Bad(t *core.T) {
	db := newTestDB(t)
	first := db.Close()
	second := db.Close()
	core.AssertNoError(t, first)
	core.AssertNoError(t, second)
}

func TestDb_DB_Close_Ugly(t *core.T) {
	db := newTestDB(t)
	core.AssertNoError(t, db.Exec("CREATE TABLE before_close(x INTEGER)"))
	core.AssertNoError(t, db.Close())
	core.AssertError(t, db.Exec("SELECT * FROM before_close"))
}

func TestDb_DB_Path_Good(t *core.T) {
	db := newTestDB(t)
	got := db.Path()
	core.AssertContains(t, got, "test.duckdb")
}

func TestDb_DB_Path_Bad(t *core.T) {
	db := &DB{path: ""}
	got := db.Path()
	core.AssertEqual(t, "", got)
}

func TestDb_DB_Path_Ugly(t *core.T) {
	db := seedMLDB(t)
	got := db.Path()
	core.AssertTrue(t, len(got) > 0)
}

func TestDb_DB_Exec_Good(t *core.T) {
	db := newTestDB(t)
	err := db.Exec("CREATE TABLE exec_good(x INTEGER)")
	core.AssertNoError(t, err)
}

func TestDb_DB_Exec_Bad(t *core.T) {
	db := newTestDB(t)
	err := db.Exec("SELECT * FROM missing_table")
	core.AssertError(t, err)
}

func TestDb_DB_Exec_Ugly(t *core.T) {
	db := newTestDB(t)
	core.RequireNoError(t, db.Exec("CREATE TABLE exec_arg(x INTEGER)"))
	err := db.Exec("INSERT INTO exec_arg VALUES (?)", 7)
	core.AssertNoError(t, err)
}

func TestDb_DB_QueryRowScan_Good(t *core.T) {
	db := newTestDB(t)
	var got int
	err := db.QueryRowScan("SELECT ?", &got, 7)
	core.RequireNoError(t, err)
	core.AssertEqual(t, 7, got)
}

func TestDb_DB_QueryRowScan_Bad(t *core.T) {
	db := newTestDB(t)
	var got int
	err := db.QueryRowScan("SELECT missing FROM nowhere", &got)
	core.AssertError(t, err)
}

func TestDb_DB_QueryRowScan_Ugly(t *core.T) {
	db := newTestDB(t)
	var got string
	err := db.QueryRowScan("SELECT 'value'", &got)
	core.RequireNoError(t, err)
	core.AssertEqual(t, "value", got)
}

func TestDb_DB_QueryGoldenSet_Good(t *core.T) {
	db := seedMLDB(t)
	rows, err := db.QueryGoldenSet(1)
	core.RequireNoError(t, err)
	core.AssertLen(t, rows, 1)
}

func TestDb_DB_QueryGoldenSet_Bad(t *core.T) {
	db := newTestDB(t)
	rows, err := db.QueryGoldenSet(1)
	core.AssertError(t, err)
	core.AssertNil(t, rows)
}

func TestDb_DB_QueryGoldenSet_Ugly(t *core.T) {
	db := seedMLDB(t)
	rows, err := db.QueryGoldenSet(999)
	core.RequireNoError(t, err)
	core.AssertEmpty(t, rows)
}

func TestDb_DB_CountGoldenSet_Good(t *core.T) {
	db := seedMLDB(t)
	count, err := db.CountGoldenSet()
	core.RequireNoError(t, err)
	core.AssertEqual(t, 1, count)
}

func TestDb_DB_CountGoldenSet_Bad(t *core.T) {
	db := newTestDB(t)
	count, err := db.CountGoldenSet()
	core.AssertError(t, err)
	core.AssertEqual(t, 0, count)
}

func TestDb_DB_CountGoldenSet_Ugly(t *core.T) {
	db := seedMLDB(t)
	core.RequireNoError(t, db.Exec("DELETE FROM golden_set"))
	count, err := db.CountGoldenSet()
	core.RequireNoError(t, err)
	core.AssertEqual(t, 0, count)
}

func TestDb_DB_QueryExpansionPrompts_Good(t *core.T) {
	db := seedMLDB(t)
	rows, err := db.QueryExpansionPrompts("pending", 1)
	core.RequireNoError(t, err)
	core.AssertLen(t, rows, 1)
}

func TestDb_DB_QueryExpansionPrompts_Bad(t *core.T) {
	db := newTestDB(t)
	rows, err := db.QueryExpansionPrompts("pending", 1)
	core.AssertError(t, err)
	core.AssertNil(t, rows)
}

func TestDb_DB_QueryExpansionPrompts_Ugly(t *core.T) {
	db := seedMLDB(t)
	rows, err := db.QueryExpansionPrompts("done", 0)
	core.RequireNoError(t, err)
	core.AssertEmpty(t, rows)
}

func TestDb_DB_CountExpansionPrompts_Good(t *core.T) {
	db := seedMLDB(t)
	total, pending, err := db.CountExpansionPrompts()
	core.RequireNoError(t, err)
	core.AssertEqual(t, 1, total)
	core.AssertEqual(t, 1, pending)
}

func TestDb_DB_CountExpansionPrompts_Bad(t *core.T) {
	db := newTestDB(t)
	total, pending, err := db.CountExpansionPrompts()
	core.AssertError(t, err)
	core.AssertEqual(t, 0, total)
	core.AssertEqual(t, 0, pending)
}

func TestDb_DB_CountExpansionPrompts_Ugly(t *core.T) {
	db := seedMLDB(t)
	core.RequireNoError(t, db.Exec("UPDATE expansion_prompts SET status = 'done'"))
	total, pending, err := db.CountExpansionPrompts()
	core.RequireNoError(t, err)
	core.AssertEqual(t, 1, total)
	core.AssertEqual(t, 0, pending)
}

func TestDb_DB_UpdateExpansionStatus_Good(t *core.T) {
	db := seedMLDB(t)
	err := db.UpdateExpansionStatus(1, "done")
	core.RequireNoError(t, err)
	rows, _ := db.QueryExpansionPrompts("done", 1)
	core.AssertLen(t, rows, 1)
}

func TestDb_DB_UpdateExpansionStatus_Bad(t *core.T) {
	db := newTestDB(t)
	err := db.UpdateExpansionStatus(1, "done")
	core.AssertError(t, err)
}

func TestDb_DB_UpdateExpansionStatus_Ugly(t *core.T) {
	db := seedMLDB(t)
	err := db.UpdateExpansionStatus(99, "done")
	core.RequireNoError(t, err)
	rows, _ := db.QueryExpansionPrompts("pending", 0)
	core.AssertLen(t, rows, 1)
}

func TestDb_DB_QueryRows_Good(t *core.T) {
	db := newTestDB(t)
	rows, err := db.QueryRows("SELECT 7 AS n")
	core.RequireNoError(t, err)
	core.AssertEqual(t, 7, toInt(rows[0]["n"]))
}

func TestDb_DB_QueryRows_Bad(t *core.T) {
	db := newTestDB(t)
	rows, err := db.QueryRows("SELECT * FROM missing_table")
	core.AssertError(t, err)
	core.AssertNil(t, rows)
}

func TestDb_DB_QueryRows_Ugly(t *core.T) {
	db := newTestDB(t)
	rows, err := db.QueryRows("SELECT ? AS value", "x")
	core.RequireNoError(t, err)
	core.AssertEqual(t, "x", rows[0]["value"])
}

func TestDb_DB_EnsureScoringTables_Good(t *core.T) {
	db := newTestDB(t)
	db.EnsureScoringTables()
	counts, err := db.TableCounts()
	core.RequireNoError(t, err)
	core.AssertContains(t, counts, TableCheckpointScores)
}

func TestDb_DB_EnsureScoringTables_Bad(t *core.T) {
	db := newTestDB(t)
	core.RequireNoError(t, db.Close())
	core.AssertNotPanics(t, func() { db.EnsureScoringTables() })
}

func TestDb_DB_EnsureScoringTables_Ugly(t *core.T) {
	db := newTestDB(t)
	db.EnsureScoringTables()
	db.EnsureScoringTables()
	core.AssertNoError(t, db.WriteScoringResult("m", "p", "suite", "dim", 1))
}

func TestDb_DB_WriteScoringResult_Good(t *core.T) {
	db := newTestDB(t)
	db.EnsureScoringTables()
	err := db.WriteScoringResult("m", "p", "suite", "dim", 1.5)
	core.AssertNoError(t, err)
}

func TestDb_DB_WriteScoringResult_Bad(t *core.T) {
	db := newTestDB(t)
	err := db.WriteScoringResult("m", "p", "suite", "dim", 1.5)
	core.AssertError(t, err)
}

func TestDb_DB_WriteScoringResult_Ugly(t *core.T) {
	db := newTestDB(t)
	db.EnsureScoringTables()
	err := db.WriteScoringResult("", "", "", "", 0)
	core.AssertNoError(t, err)
}

func TestDb_DB_TableCounts_Good(t *core.T) {
	db := seedMLDB(t)
	counts, err := db.TableCounts()
	core.RequireNoError(t, err)
	core.AssertEqual(t, 1, counts["golden_set"])
}

func TestDb_DB_TableCounts_Bad(t *core.T) {
	db := newTestDB(t)
	counts, err := db.TableCounts()
	core.RequireNoError(t, err)
	core.AssertEmpty(t, counts)
}

func TestDb_DB_TableCounts_Ugly(t *core.T) {
	db := newTestDB(t)
	db.EnsureScoringTables()
	counts, err := db.TableCounts()
	core.RequireNoError(t, err)
	core.AssertContains(t, counts, "scoring_results")
}
