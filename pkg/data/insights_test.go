package data

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInsightsSummary_EmptyDB(t *testing.T) {
	db := setupTestDB(t)

	summary, err := GetInsightsSummary(db, nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.BusFactor)
	assert.Equal(t, 0, summary.PonyFactor)
}

func TestGetInsightsSummary_NilDB(t *testing.T) {
	_, err := GetInsightsSummary(nil, nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetInsightsSummary_WithData(t *testing.T) {
	db := setupTestDB(t)

	// Insert developers
	_, err := db.Exec(`INSERT INTO developer (username, full_name, entity) VALUES
		('alice', 'Alice', 'ACME'),
		('bob', 'Bob', 'ACME'),
		('carol', 'Carol', 'BETA')`)
	require.NoError(t, err)

	// Insert events: alice has 50 events, bob has 30, carol has 20
	// Bus factor should be 1 (alice alone covers 50%)
	for i := 0; i < 50; i++ {
		_, err = db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels)
			VALUES ('org1', 'repo1', 'alice', 'pr', ?, 'http://a', '', '')`,
			"2025-01-"+padDay(i))
		require.NoError(t, err)
	}
	for i := 0; i < 30; i++ {
		_, err = db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels)
			VALUES ('org1', 'repo1', 'bob', 'pr', ?, 'http://b', '', '')`,
			"2025-01-"+padDay(i))
		require.NoError(t, err)
	}
	for i := 0; i < 20; i++ {
		_, err = db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels)
			VALUES ('org1', 'repo1', 'carol', 'issue', ?, 'http://c', '', '')`,
			"2025-01-"+padDay(i))
		require.NoError(t, err)
	}

	summary, err := GetInsightsSummary(db, nil, nil, nil, 24)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, summary.BusFactor, 1)
	assert.GreaterOrEqual(t, summary.PonyFactor, 1)
}

func TestGetContributorRetention_EmptyDB(t *testing.T) {
	db := setupTestDB(t)

	series, err := GetContributorRetention(db, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetContributorRetention_NilDB(t *testing.T) {
	_, err := GetContributorRetention(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetContributorRetention_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice'), ('bob', 'Bob')`)
	require.NoError(t, err)

	// Alice appears in Jan and Feb; Bob appears only in Feb.
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels) VALUES
		('org1', 'repo1', 'alice', 'pr', '2025-01-15', 'http://a', '', ''),
		('org1', 'repo1', 'alice', 'issue', '2025-02-15', 'http://a2', '', ''),
		('org1', 'repo1', 'bob', 'pr', '2025-02-15', 'http://b', '', '')`)
	require.NoError(t, err)

	series, err := GetContributorRetention(db, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 2)

	// January: alice is new
	assert.Equal(t, "2025-01", series.Months[0])
	assert.Equal(t, 1, series.New[0])
	assert.Equal(t, 0, series.Returning[0])

	// February: bob is new, alice is returning
	assert.Equal(t, "2025-02", series.Months[1])
	assert.Equal(t, 1, series.New[1])
	assert.Equal(t, 1, series.Returning[1])
}

func TestGetPRReviewRatio_EmptyDB(t *testing.T) {
	db := setupTestDB(t)

	series, err := GetPRReviewRatio(db, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetPRReviewRatio_NilDB(t *testing.T) {
	_, err := GetPRReviewRatio(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetPRReviewRatio_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice'), ('bob', 'Bob')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels) VALUES
		('org1', 'repo1', 'alice', 'pr', '2025-01-10', 'http://a', '', ''),
		('org1', 'repo1', 'alice', 'pr', '2025-01-11', 'http://a2', '', ''),
		('org1', 'repo1', 'bob', 'pr_review', '2025-01-10', 'http://b', '', ''),
		('org1', 'repo1', 'bob', 'pr_review', '2025-01-11', 'http://b2', '', ''),
		('org1', 'repo1', 'bob', 'pr_review', '2025-01-12', 'http://b3', '', '')`)
	require.NoError(t, err)

	series, err := GetPRReviewRatio(db, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1)
	assert.Equal(t, 2, series.PRs[0])
	assert.Equal(t, 3, series.Reviews[0])
	assert.InDelta(t, 1.5, series.Ratio[0], 0.01)
}

func TestGetTimeToMerge_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	series, err := GetTimeToMerge(db, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetTimeToMerge_NilDB(t *testing.T) {
	_, err := GetTimeToMerge(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetTimeToMerge_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, merged_at)
		VALUES
		('org1', 'repo1', 'alice', 'pr', '2025-01-15', 'http://a', '', '', 'closed', '2025-01-10T00:00:00Z', '2025-01-15T00:00:00Z'),
		('org1', 'repo1', 'alice', 'pr', '2025-01-20', 'http://a2', '', '', 'closed', '2025-01-18T00:00:00Z', '2025-01-20T00:00:00Z')`)
	require.NoError(t, err)

	series, err := GetTimeToMerge(db, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1)
	assert.Equal(t, 2, series.Count[0])
	assert.InDelta(t, 3.5, series.AvgDays[0], 0.01) // (5+2)/2
}

func TestGetTimeToClose_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	series, err := GetTimeToClose(db, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetTimeToClose_NilDB(t *testing.T) {
	_, err := GetTimeToClose(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetTimeToClose_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, closed_at)
		VALUES
		('org1', 'repo1', 'alice', 'issue', '2025-01-15', 'http://a', '', '', 'closed', '2025-01-10T00:00:00Z', '2025-01-15T00:00:00Z'),
		('org1', 'repo1', 'alice', 'issue', '2025-01-20', 'http://a2', '', '', 'closed', '2025-01-14T00:00:00Z', '2025-01-20T00:00:00Z')`)
	require.NoError(t, err)

	series, err := GetTimeToClose(db, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1)
	assert.Equal(t, 2, series.Count[0])
	assert.InDelta(t, 5.5, series.AvgDays[0], 0.01) // (5+6)/2
}

func padDay(i int) string {
	return fmt.Sprintf("%02d", (i%28)+1)
}
