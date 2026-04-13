package ml

import (
	"dappco.re/go/core"
	"io"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/store"
)

// SeedInfluxConfig holds options for the SeedInflux migration.
type SeedInfluxConfig struct {
	Force     bool
	BatchSize int
}

// SeedInflux migrates golden_set rows from DuckDB into InfluxDB as
// gold_gen measurement points. This is a one-time migration tool;
// it skips the write when InfluxDB already contains all records
// unless Force is set.
func SeedInflux(db *store.DuckDB, influx *InfluxClient, cfg SeedInfluxConfig, w io.Writer) error {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}

	// Count source rows in DuckDB.
	var total int
	if err := db.Conn().QueryRow("SELECT count(*) FROM golden_set").Scan(&total); err != nil {
		return coreerr.E("ml.SeedInflux", "no golden_set table", err)
	}

	// Check how many distinct records InfluxDB already has.
	existing := 0
	rows, err := influx.QuerySQL("SELECT count(DISTINCT i) AS n FROM gold_gen")
	if err == nil && len(rows) > 0 {
		if n, ok := rows[0]["n"].(float64); ok {
			existing = int(n)
		}
	}

	fprintf(w, "DuckDB has %d records, InfluxDB golden_gen has %d\n", total, existing)

	if existing >= total && !cfg.Force {
		fprintf(w, "%s\n", "InfluxDB already has all records. Use --force to re-seed.")
		return nil
	}

	// Query all golden_set rows from DuckDB.
	dbRows, err := db.Conn().Query(
		"SELECT idx, seed_id, domain, voice, gen_time, char_count FROM golden_set ORDER BY idx",
	)
	if err != nil {
		return coreerr.E("ml.SeedInflux", "query golden_set", err)
	}
	defer dbRows.Close()

	var batch []string
	written := 0

	for dbRows.Next() {
		var idx int
		var seedID, domain, voice string
		var genTime float64
		var charCount int

		if err := dbRows.Scan(&idx, &seedID, &domain, &voice, &genTime, &charCount); err != nil {
			return coreerr.E("ml.SeedInflux", core.Sprintf("scan row %d", written), err)
		}

		// Build line protocol point.
		// Tags: i (idx), w (worker), d (domain), v (voice)
		// Fields: seed_id (string), gen_time (float), chars (integer)
		escapedSeedID := replaceAll(seedID, `"`, `\"`)

		line := core.Sprintf(
			"gold_gen,i=%s,w=migration,d=%s,v=%s seed_id=\"%s\",gen_time=%v,chars=%di",
			EscapeLp(core.Sprintf("%d", idx)),
			EscapeLp(domain),
			EscapeLp(voice),
			escapedSeedID,
			genTime,
			charCount,
		)
		batch = append(batch, line)

		if len(batch) >= cfg.BatchSize {
			if err := influx.WriteLp(batch); err != nil {
				return coreerr.E("ml.SeedInflux", core.Sprintf("write batch at row %d", written), err)
			}
			written += len(batch)
			batch = batch[:0]

			if written%2000 == 0 {
				fprintf(w, "  wrote %d / %d\n", written, total)
			}
		}
	}

	if err := dbRows.Err(); err != nil {
		return coreerr.E("ml.SeedInflux", "iterate golden_set rows", err)
	}

	// Flush remaining batch.
	if len(batch) > 0 {
		if err := influx.WriteLp(batch); err != nil {
			return coreerr.E("ml.SeedInflux", "write final batch", err)
		}
		written += len(batch)
	}

	fprintf(w, "Seeded %d records into InfluxDB golden_gen\n", written)
	return nil
}
