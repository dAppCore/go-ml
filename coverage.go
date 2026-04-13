package ml

import (
	"io"

	"dappco.re/go/core"
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

<<<<<<< HEAD
	fprintf(w, "%s\n", "LEM Seed Coverage Analysis")
	fprintf(w, "%s\n", "==================================================")
	fprintf(w, "\nTotal seeds: %d\n", total)
=======
	core.Print(w, "LEM Seed Coverage Analysis")
	core.Print(w, "==================================================")
	core.Print(w, "")
	core.Print(w, "Total seeds: %d", total)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

	// Region distribution.
	regionRows, err := queryRegionDistribution(db)
	if err != nil {
		return coreerr.E("ml.PrintCoverage", "query regions", err)
	}

<<<<<<< HEAD
	fprintf(w, "%s\n", "\nRegion distribution (underrepresented first):")
	avg := float64(total) / float64(len(regionRows))
	for _, r := range regionRows {
		barLen := min(int(float64(r.n)/avg*10), 40)
		bar := repeatStr("#", barLen)
=======
	core.Print(w, "")
	core.Print(w, "Region distribution (underrepresented first):")
	avg := float64(total) / float64(len(regionRows))
	for _, r := range regionRows {
		barLen := min(int(float64(r.n)/avg*10), 40)
		bar := repeatString("#", barLen)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
		gap := ""
		if float64(r.n) < avg*0.5 {
			gap = "  <- UNDERREPRESENTED"
		}
<<<<<<< HEAD
		fprintf(w, "  %-22s %6d  (%4d domains)  %s%s\n", r.group, r.n, r.domains, bar, gap)
	}

	// Top 10 domains.
	fprintf(w, "%s\n", "\nTop 10 domains (most seeds):")
=======
		core.Print(w, "  %-22s %6d  (%4d domains)  %s%s", r.group, r.n, r.domains, bar, gap)
	}

	// Top 10 domains.
	core.Print(w, "")
	core.Print(w, "Top 10 domains (most seeds):")
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	topRows, err := db.QueryRows(`
		SELECT domain, count(*) AS n FROM seeds
		WHERE domain != '' GROUP BY domain ORDER BY n DESC LIMIT 10
	`)
	if err == nil {
		for _, row := range topRows {
			domain := strVal(row, "domain")
			n := toInt(row["n"])
<<<<<<< HEAD
			fprintf(w, "  %-40s %5d\n", domain, n)
=======
			core.Print(w, "  %-40s %5d", domain, n)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
		}
	}

	// Bottom 10 domains.
<<<<<<< HEAD
	fprintf(w, "%s\n", "\nBottom 10 domains (fewest seeds, min 5):")
=======
	core.Print(w, "")
	core.Print(w, "Bottom 10 domains (fewest seeds, min 5):")
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	bottomRows, err := db.QueryRows(`
		SELECT domain, count(*) AS n FROM seeds
		WHERE domain != '' GROUP BY domain HAVING count(*) >= 5 ORDER BY n ASC LIMIT 10
	`)
	if err == nil {
		for _, row := range bottomRows {
			domain := strVal(row, "domain")
			n := toInt(row["n"])
<<<<<<< HEAD
			fprintf(w, "  %-40s %5d\n", domain, n)
		}
	}

	fprintf(w, "%s\n", "\nSuggested expansion areas:")
	fprintf(w, "%s\n", "  - Japanese, Korean, Thai, Vietnamese (no seeds found)")
	fprintf(w, "%s\n", "  - Hindi/Urdu, Bengali, Tamil (South Asian)")
	fprintf(w, "%s\n", "  - Swahili, Yoruba, Amharic (Sub-Saharan Africa)")
	fprintf(w, "%s\n", "  - Indigenous languages (Quechua, Nahuatl, Aymara)")
=======
			core.Print(w, "  %-40s %5d", domain, n)
		}
	}

	core.Print(w, "")
	core.Print(w, "Suggested expansion areas:")
	core.Print(w, "  - Japanese, Korean, Thai, Vietnamese (no seeds found)")
	core.Print(w, "  - Hindi/Urdu, Bengali, Tamil (South Asian)")
	core.Print(w, "  - Swahili, Yoruba, Amharic (Sub-Saharan Africa)")
	core.Print(w, "  - Indigenous languages (Quechua, Nahuatl, Aymara)")
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa

	return nil
}

func repeatString(part string, count int) string {
	if count <= 0 {
		return ""
	}
	b := core.NewBuilder()
	for range count {
		b.WriteString(part)
	}
	return b.String()
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
