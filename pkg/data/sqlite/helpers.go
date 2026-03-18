package sqlite

import (
	"database/sql"
	"log/slog"
	"regexp"
	"time"
)

const (
	nonAlphaNumRegex string = "[^a-zA-Z0-9 ]+"

	// botExcludeSQL filters out bot accounts using the "e" table alias.
	botExcludeSQL = `AND e.username NOT LIKE '%[bot]'
		AND LOWER(e.username) NOT IN ('copilot','github-copilot','claude','anthropic-claude')`

	// botExcludeDSQL filters out bot accounts using the "d" table alias.
	botExcludeDSQL = `AND d.username NOT LIKE '%[bot]'
		AND LOWER(d.username) NOT IN ('copilot','github-copilot','claude','anthropic-claude')`

	// botExcludePrSQL filters out bot accounts using the "pr" table alias.
	botExcludePrSQL = `AND pr.username NOT LIKE '%[bot]'
		AND LOWER(pr.username) NOT IN ('copilot','github-copilot','claude','anthropic-claude')`

	// forkExcludeSQL excludes fork events from the join so only code/comment
	// activity (PR, PR review, issue, issue comment) counts toward reputation.
	// Uses "e" as the event table alias.
	forkExcludeSQL = `AND e.type != 'fork'`
)

var (
	entityRegEx = regexp.MustCompile(nonAlphaNumRegex)

	entityNoise = map[string]bool{
		"B.V.":        true,
		"CDL":         true,
		"CO":          true,
		"COMPANY":     true,
		"CORP":        true,
		"CORPORATION": true,
		"GMBH":        true,
		"GROUP":       true,
		"INC":         true,
		"LLC":         true,
		"LC":          true,
		"P.C.":        true,
		"P.A.":        true,
		"S.C.":        true,
		"LTD.":        true,
		"CHTD.":       true,
		"PC":          true,
		"LTD":         true,
		"PVT":         true,
		"SE":          true,
		"S.A.":        true,
	}

	entitySubstitutions = map[string]string{
		"CHAINGUARDDEV":       "CHAINGUARD",
		"GCP":                 "GOOGLE",
		"GOOGLECLOUD":         "GOOGLE",
		"GOOGLECLOUDPLATFORM": "GOOGLE",
		"HUAWEICLOUD":         "HUAWEI",
		"IBM CODAITY":         "IBM",
		"IBM RESEARCH":        "IBM",
		"INTERNATIONAL BUSINESS MACHINES CORPORATION":                 "IBM",
		"INTERNATIONAL BUSINESS MACHINES":                             "IBM",
		"INTERNATIONAL INSTITUTE OF INFORMATION TECHNOLOGY BANGALORE": "IIIT BANGALORE",
		"LINE PLUS":       "LINE",
		"MICROSOFT CHINA": "MICROSOFT",
		"REDHATOFFICIAL":  "REDHAT",
		"S&P GLOBAL INC":  "S&P",
		"S&P GLOBAL":      "S&P",
		"VERVERICA ORIGINAL CREATORS OF APACHE FLINK": "VERVERICA",
	}
)

func sinceDate(months int) string {
	return time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")
}

func rollbackTransaction(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil {
		slog.Error("error rolling back transaction", "error", err)
	}
}
