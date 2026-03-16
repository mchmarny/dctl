package data

import (
	"errors"
	"regexp"
	"time"
)

const (
	nonAlphaNumRegex string = "[^a-zA-Z0-9 ]+"

	// botExcludeSQL filters out bot accounts using the "e" table alias.
	botExcludeSQL = `AND e.username NOT LIKE '%[bot]'
		AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')`

	// botExcludeDSQL filters out bot accounts using the "d" table alias (with LOWER).
	botExcludeDSQL = `AND d.username NOT LIKE '%[bot]'
		AND LOWER(d.username) NOT IN ('copilot','github-copilot','claude','anthropic-claude')`

	// botExcludePrSQL filters out bot accounts using the "pr" table alias.
	botExcludePrSQL = `AND pr.username NOT LIKE '%[bot]'
		AND pr.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')`
)

var (
	ErrDBNotInitialized = errors.New("database not initialized")

	entityRegEx = regexp.MustCompile(nonAlphaNumRegex)
)

func sinceDate(months int) string {
	return time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")
}

// Contains checks for val in list
func Contains[T comparable](list []T, val T) bool {
	if list == nil {
		return false
	}
	for _, item := range list {
		if item == val {
			return true
		}
	}
	return false
}
