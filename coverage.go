package ml

import (
	"io"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/store"
)

// regionRow holds a single row from the region distribution query.
type regionRow struct {
	group   string
	n       int
	domains int
}

// PrintCoverage analyzes seed coverage by region and domain, printing
// a report with bar chart visualization and gap recommendations.
func PrintCoverage(db *store.DuckDB, w io.Writer) error {
	rows, err := db.QueryRows("SELECT count(*) AS total FROM seeds")
	if err != nil {
		return coreerr.E("ml.PrintCoverage", "count seeds", err)
	}
	if len(rows) == 0 {
		return coreerr.E("ml.PrintCoverage", "no seeds table found (run: core ml import-all first)", nil)
	}
	total := toInt(rows[0]["total"])

	fprintf(w, "%s\n", "LEM Seed Coverage Analysis")
	fprintf(w, "%s\n", "==================================================")
	fprintf(w, "\nTotal seeds: %d\n", total)

	// Region distribution.
	regionRows, err := queryRegionDistribution(db)
	if err != nil {
		return coreerr.E("ml.PrintCoverage", "query regions", err)
	}

	fprintf(w, "%s\n", "\nRegion distribution (underrepresented first):")
	avg := float64(total) / float64(len(regionRows))
	for _, r := range regionRows {
		barLen := min(int(float64(r.n)/avg*10), 40)
		bar := repeatStr("#", barLen)
		gap := ""
		if float64(r.n) < avg*0.5 {
			gap = "  <- UNDERREPRESENTED"
		}
		fprintf(w, "  %-22s %6d  (%4d domains)  %s%s\n", r.group, r.n, r.domains, bar, gap)
	}

	// Top 10 domains.
	fprintf(w, "%s\n", "\nTop 10 domains (most seeds):")
	topRows, err := db.QueryRows(`
		SELECT domain, count(*) AS n FROM seeds
		WHERE domain != '' GROUP BY domain ORDER BY n DESC LIMIT 10
	`)
	if err == nil {
		for _, row := range topRows {
			domain := strVal(row, "domain")
			n := toInt(row["n"])
			fprintf(w, "  %-40s %5d\n", domain, n)
		}
	}

	// Bottom 10 domains.
	fprintf(w, "%s\n", "\nBottom 10 domains (fewest seeds, min 5):")
	bottomRows, err := db.QueryRows(`
		SELECT domain, count(*) AS n FROM seeds
		WHERE domain != '' GROUP BY domain HAVING count(*) >= 5 ORDER BY n ASC LIMIT 10
	`)
	if err == nil {
		for _, row := range bottomRows {
			domain := strVal(row, "domain")
			n := toInt(row["n"])
			fprintf(w, "  %-40s %5d\n", domain, n)
		}
	}

	fprintf(w, "%s\n", "\nSuggested expansion areas:")
	fprintf(w, "%s\n", "  - Japanese, Korean, Thai, Vietnamese (no seeds found)")
	fprintf(w, "%s\n", "  - Hindi/Urdu, Bengali, Tamil (South Asian)")
	fprintf(w, "%s\n", "  - Swahili, Yoruba, Amharic (Sub-Saharan Africa)")
	fprintf(w, "%s\n", "  - Indigenous languages (Quechua, Nahuatl, Aymara)")

	return nil
}

// queryRegionDistribution returns seed counts grouped by normalized language
// region, ordered ascending (underrepresented first).
func queryRegionDistribution(db *store.DuckDB) ([]regionRow, error) {
	rows, err := db.QueryRows(`
		SELECT
			CASE
				WHEN region LIKE '%cn%' THEN 'cn (Chinese)'
				WHEN region LIKE '%en-%' OR region LIKE '%en_para%' OR region LIKE '%para%' THEN 'en (English)'
				WHEN region LIKE '%ru%' THEN 'ru (Russian)'
				WHEN region LIKE '%de%' AND region NOT LIKE '%deten%' THEN 'de (German)'
				WHEN region LIKE '%es%' THEN 'es (Spanish)'
				WHEN region LIKE '%fr%' THEN 'fr (French)'
				WHEN region LIKE '%latam%' THEN 'latam (LatAm)'
				WHEN region LIKE '%africa%' THEN 'africa'
				WHEN region LIKE '%eu%' THEN 'eu (European)'
				WHEN region LIKE '%me%' AND region NOT LIKE '%premium%' THEN 'me (MidEast)'
				WHEN region LIKE '%multi%' THEN 'multilingual'
				WHEN region LIKE '%weak%' THEN 'weak-langs'
				ELSE 'other'
			END AS lang_group,
			count(*) AS n,
			count(DISTINCT domain) AS domains
		FROM seeds GROUP BY lang_group ORDER BY n ASC
	`)
	if err != nil {
		return nil, err
	}

	result := make([]regionRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, regionRow{
			group:   strVal(row, "lang_group"),
			n:       toInt(row["n"]),
			domains: toInt(row["domains"]),
		})
	}
	return result, nil
}
