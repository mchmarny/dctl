package sqlite

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInsightsSummary_EmptyDB(t *testing.T) {
	store := setupTestDB(t)

	summary, err := store.GetInsightsSummary(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.BusFactor)
	assert.Equal(t, 0, summary.PonyFactor)
	assert.Equal(t, 0, summary.Orgs)
	assert.Equal(t, 0, summary.Events)
	assert.Empty(t, summary.LastImport)
}

func TestGetInsightsSummary_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetInsightsSummary(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetInsightsSummary_WithData(t *testing.T) {
	store := setupTestDB(t)

	// Insert repo meta
	_, err := store.db.Exec(`INSERT INTO repo_meta (org, repo, stars, forks, open_issues, language, license, archived, last_import_at)
		VALUES ('org1', 'repo1', 10, 5, 1, 'Go', 'MIT', 0, '2025-01-31T12:00:00Z')`)
	require.NoError(t, err)

	// Insert developers
	_, err = store.db.Exec(`INSERT INTO developer (username, full_name, entity) VALUES
		('alice', 'Alice', 'ACME'),
		('bob', 'Bob', 'ACME'),
		('carol', 'Carol', 'BETA')`)
	require.NoError(t, err)

	// Insert events: alice has 50 events, bob has 30, carol has 20
	for i := 0; i < 50; i++ {
		_, err = store.db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels)
			VALUES ('org1', 'repo1', 'alice', 'pr', ?, 'http://a', '', '')`,
			"2025-01-"+padDay(i))
		require.NoError(t, err)
	}
	for i := 0; i < 30; i++ {
		_, err = store.db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels)
			VALUES ('org1', 'repo1', 'bob', 'pr', ?, 'http://b', '', '')`,
			"2025-01-"+padDay(i))
		require.NoError(t, err)
	}
	for i := 0; i < 20; i++ {
		_, err = store.db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels)
			VALUES ('org1', 'repo1', 'carol', 'issue', ?, 'http://c', '', '')`,
			"2025-01-"+padDay(i))
		require.NoError(t, err)
	}

	summary, err := store.GetInsightsSummary(nil, nil, nil, 24)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, summary.BusFactor, 1)
	assert.GreaterOrEqual(t, summary.PonyFactor, 1)
	assert.Greater(t, summary.Events, 0)
	assert.Greater(t, summary.Contributors, 0)
	assert.Greater(t, summary.Orgs, 0)
	assert.Greater(t, summary.Repos, 0)
	assert.NotEmpty(t, summary.LastImport)
}

func TestGetContributorRetention_EmptyDB(t *testing.T) {
	store := setupTestDB(t)

	series, err := store.GetContributorRetention(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetContributorRetention_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetContributorRetention(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetContributorRetention_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice'), ('bob', 'Bob')`)
	require.NoError(t, err)

	// Alice appears in Jan and Feb; Bob appears only in Feb.
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels) VALUES
		('org1', 'repo1', 'alice', 'pr', '2025-01-15', 'http://a', '', ''),
		('org1', 'repo1', 'alice', 'issue', '2025-02-15', 'http://a2', '', ''),
		('org1', 'repo1', 'bob', 'pr', '2025-02-15', 'http://b', '', '')`)
	require.NoError(t, err)

	series, err := store.GetContributorRetention(nil, nil, nil, 24)
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
	store := setupTestDB(t)

	series, err := store.GetPRReviewRatio(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetPRReviewRatio_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetPRReviewRatio(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetPRReviewRatio_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice'), ('bob', 'Bob')`)
	require.NoError(t, err)

	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels) VALUES
		('org1', 'repo1', 'alice', 'pr', '2025-01-10', 'http://a', '', ''),
		('org1', 'repo1', 'alice', 'pr', '2025-01-11', 'http://a2', '', ''),
		('org1', 'repo1', 'bob', 'pr_review', '2025-01-10', 'http://b', '', ''),
		('org1', 'repo1', 'bob', 'pr_review', '2025-01-11', 'http://b2', '', ''),
		('org1', 'repo1', 'bob', 'pr_review', '2025-01-12', 'http://b3', '', '')`)
	require.NoError(t, err)

	series, err := store.GetPRReviewRatio(nil, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1)
	assert.Equal(t, 2, series.PRs[0])
	assert.Equal(t, 3, series.Reviews[0])
	assert.InDelta(t, 1.5, series.Ratio[0], 0.01)
}

func TestGetTimeToMerge_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	series, err := store.GetTimeToMerge(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetTimeToMerge_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetTimeToMerge(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetTimeToMerge_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, merged_at)
		VALUES
		('org1', 'repo1', 'alice', 'pr', '2025-01-15', 'http://a', '', '', 'closed', '2025-01-10T00:00:00Z', '2025-01-15T00:00:00Z'),
		('org1', 'repo1', 'alice', 'pr', '2025-01-20', 'http://a2', '', '', 'closed', '2025-01-18T00:00:00Z', '2025-01-20T00:00:00Z')`)
	require.NoError(t, err)

	series, err := store.GetTimeToMerge(nil, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1)
	assert.Equal(t, 2, series.Count[0])
	assert.InDelta(t, 3.5, series.AvgDays[0], 0.01) // (5+2)/2
}

func TestGetTimeToClose_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	series, err := store.GetTimeToClose(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetTimeToClose_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetTimeToClose(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetTimeToClose_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, closed_at)
		VALUES
		('org1', 'repo1', 'alice', 'issue', '2025-01-15', 'http://a', '', '', 'closed', '2025-01-10T00:00:00Z', '2025-01-15T00:00:00Z'),
		('org1', 'repo1', 'alice', 'issue', '2025-01-20', 'http://a2', '', '', 'closed', '2025-01-14T00:00:00Z', '2025-01-20T00:00:00Z')`)
	require.NoError(t, err)

	series, err := store.GetTimeToClose(nil, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1)
	assert.Equal(t, 2, series.Count[0])
	assert.InDelta(t, 5.5, series.AvgDays[0], 0.01) // (5+6)/2
}

func TestGetTimeToRestoreBugs_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetTimeToRestoreBugs(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetTimeToRestoreBugs_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	series, err := store.GetTimeToRestoreBugs(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetTimeToRestoreBugs_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	// Insert a release
	_, err = store.db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease)
		VALUES ('org1', 'repo1', 'v1.0', 'v1.0', '2025-01-15T00:00:00Z', 0)`)
	require.NoError(t, err)

	// Bug issue near release, closed in 1 day
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, closed_at)
		VALUES ('org1', 'repo1', 'alice', 'issue', '2025-01-17', 'http://a', '', 'bug', 'closed', '2025-01-17T10:00:00Z', '2025-01-18T10:00:00Z')`)
	require.NoError(t, err)

	// Non-bug issue, closed in 3 days (should NOT be included)
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, closed_at)
		VALUES ('org1', 'repo1', 'alice', 'issue', '2025-01-18', 'http://b', '', 'enhancement', 'closed', '2025-01-18T10:00:00Z', '2025-01-21T10:00:00Z')`)
	require.NoError(t, err)

	// Bug issue NOT near any release (30 days later), should NOT be included
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, closed_at)
		VALUES ('org1', 'repo1', 'alice', 'issue', '2025-02-17', 'http://c', '', 'bug', 'closed', '2025-02-17T10:00:00Z', '2025-02-20T10:00:00Z')`)
	require.NoError(t, err)

	series, err := store.GetTimeToRestoreBugs(nil, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1) // Only January has a qualifying bug
	assert.Equal(t, "2025-01", series.Months[0])
	assert.Equal(t, 1, series.Count[0])
	assert.InDelta(t, 1.0, series.AvgDays[0], 0.01) // 1 day
}

func TestGetChangeFailureRate_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetChangeFailureRate(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetChangeFailureRate_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	series, err := store.GetChangeFailureRate(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetChangeFailureRate_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	// Insert a release (deployment)
	_, err = store.db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease)
		VALUES ('org1', 'repo1', 'v1.0', 'v1.0', '2025-01-15T00:00:00Z', 0)`)
	require.NoError(t, err)

	// Insert a bug issue within 7 days of release
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, title)
		VALUES ('org1', 'repo1', 'alice', 'issue', '2025-01-17', 'http://a', '', 'bug', 'open', '2025-01-17T10:00:00Z', 'Bug in feature')`)
	require.NoError(t, err)

	// Insert a revert PR in same month
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, title)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-16', 'http://b', '', '', 'merged', '2025-01-16T10:00:00Z', 'Revert "Add feature"')`)
	require.NoError(t, err)

	series, err := store.GetChangeFailureRate(nil, nil, nil, 24)
	require.NoError(t, err)
	require.NotEmpty(t, series.Months)
	assert.Equal(t, "2025-01", series.Months[0])
	assert.Equal(t, 2, series.Failures[0])
	assert.Equal(t, 1, series.Deployments[0])
	assert.InDelta(t, 200.0, series.Rate[0], 0.1) // 2 failures / 1 deployment * 100
}

func TestGetReviewLatency_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetReviewLatency(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetReviewLatency_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	series, err := store.GetReviewLatency(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetReviewLatency_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice'), ('bob', 'Bob')`)
	require.NoError(t, err)

	// PR created by alice
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, number, created_at)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-10', 'http://a', '', '', 'open', 42, '2025-01-10T10:00:00Z')`)
	require.NoError(t, err)

	// First review by bob, 6 hours later
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, number, created_at)
		VALUES ('org1', 'repo1', 'bob', 'pr_review', '2025-01-10', 'http://b', '', '', 42, '2025-01-10T16:00:00Z')`)
	require.NoError(t, err)

	// Second review by bob, 12 hours later (should NOT affect -- we take MIN)
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, number, created_at)
		VALUES ('org1', 'repo1', 'bob', 'pr_review', '2025-01-11', 'http://c', '', '', 42, '2025-01-10T22:00:00Z')`)
	require.NoError(t, err)

	series, err := store.GetReviewLatency(nil, nil, nil, 24)
	require.NoError(t, err)
	require.NotEmpty(t, series.Months)

	// Find January in results
	found := false
	for i, m := range series.Months {
		if m == "2025-01" {
			found = true
			assert.Equal(t, 1, series.Count[i])
			assert.InDelta(t, 6.0, series.AvgHours[i], 0.1) // 6 hours
			break
		}
	}
	assert.True(t, found, "expected 2025-01 in results")
}

func TestGetPRSizeDistribution_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetPRSizeDistribution(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetPRSizeDistribution_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	series, err := store.GetPRSizeDistribution(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetPRSizeDistribution_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	// Small PR (20 lines)
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, additions, deletions)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-10', 'http://a', '', '', 'merged', '2025-01-10T10:00:00Z', 15, 5)`)
	require.NoError(t, err)

	// Medium PR (100 lines)
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, additions, deletions)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-11', 'http://b', '', '', 'merged', '2025-01-11T10:00:00Z', 70, 30)`)
	require.NoError(t, err)

	// Large PR (500 lines)
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, additions, deletions)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-12', 'http://c', '', '', 'merged', '2025-01-12T10:00:00Z', 400, 100)`)
	require.NoError(t, err)

	// XL PR (1500 lines)
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, additions, deletions)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-13', 'http://d', '', '', 'merged', '2025-01-13T10:00:00Z', 1000, 500)`)
	require.NoError(t, err)

	series, err := store.GetPRSizeDistribution(nil, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1)
	assert.Equal(t, "2025-01", series.Months[0])
	assert.Equal(t, 1, series.Small[0])
	assert.Equal(t, 1, series.Medium[0])
	assert.Equal(t, 1, series.Large[0])
	assert.Equal(t, 1, series.XLarge[0])
}

func TestGetContributorMomentum_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetContributorMomentum(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetContributorMomentum_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	series, err := store.GetContributorMomentum(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetContributorMomentum_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice'), ('bob', 'Bob')`)
	require.NoError(t, err)

	// Both active in Jan 2025
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-10', 'http://a', '', ''),
		       ('org1', 'repo1', 'bob', 'pr', '2025-01-11', 'http://b', '', '')`)
	require.NoError(t, err)

	// Only alice in Feb 2025
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-02-10', 'http://c', '', '')`)
	require.NoError(t, err)

	series, err := store.GetContributorMomentum(nil, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 2)

	// Jan: 2 active (alice + bob)
	assert.Equal(t, "2025-01", series.Months[0])
	assert.Equal(t, 2, series.Active[0])
	assert.Equal(t, 0, series.Delta[0]) // first month, delta=0

	// Feb: still 2 active (rolling 3-month window includes Jan, so bob is still counted)
	assert.Equal(t, "2025-02", series.Months[1])
	assert.Equal(t, 2, series.Active[1])
	assert.Equal(t, 0, series.Delta[1])
}

func TestGetContributorFunnel_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetContributorFunnel(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetContributorFunnel_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	series, err := store.GetContributorFunnel(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetContributorFunnel_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice'), ('bob', 'Bob')`)
	require.NoError(t, err)

	// Alice: first comment, first PR, first merge -- all in Jan
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, merged_at)
		VALUES
		('org1', 'repo1', 'alice', 'issue_comment', '2025-01-05', 'http://a', '', '', NULL, '2025-01-05T10:00:00Z', NULL),
		('org1', 'repo1', 'alice', 'pr', '2025-01-10', 'http://b', '', '', 'merged', '2025-01-10T10:00:00Z', '2025-01-10T12:00:00Z')`)
	require.NoError(t, err)

	// Bob: first comment only in Jan
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'bob', 'issue_comment', '2025-01-08', 'http://c', '', '')`)
	require.NoError(t, err)

	series, err := store.GetContributorFunnel(nil, nil, nil, 24)
	require.NoError(t, err)
	require.NotEmpty(t, series.Months)

	// Find January
	found := false
	for i, m := range series.Months {
		if m == "2025-01" {
			found = true
			assert.Equal(t, 2, series.FirstComment[i]) // Alice + Bob
			assert.Equal(t, 1, series.FirstPR[i])      // Alice only
			assert.Equal(t, 1, series.FirstMerge[i])   // Alice only
			break
		}
	}
	assert.True(t, found, "expected 2025-01 in results")
}

func TestGetContributorProfile_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetContributorProfile("alice", nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetContributorProfile_EmptyUsername(t *testing.T) {
	store := setupTestDB(t)
	_, err := store.GetContributorProfile("", nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetContributorProfile_EmptyDB(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name, entity) VALUES ('alice', 'Alice', 'ACME')`)
	require.NoError(t, err)

	series, err := store.GetContributorProfile("alice", nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Len(t, series.Metrics, 9)
	assert.Len(t, series.Values, 9)
	assert.Len(t, series.Averages, 9)
	for _, v := range series.Values {
		assert.Equal(t, 0, v)
	}
}

func TestGetContributorProfile_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name, entity) VALUES
		('alice', 'Alice', 'ACME'),
		('bob', 'Bob', 'ACME')`)
	require.NoError(t, err)

	// alice: 3 PRs (open), 1 merged PR, 2 issues
	for i := 0; i < 3; i++ {
		_, err = store.db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels, state)
			VALUES ('org1', 'repo1', 'alice', 'pr', ?, 'http://a', '', '', 'open')`,
			"2026-01-"+padDay(i))
		require.NoError(t, err)
	}
	_, err = store.db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels, state)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2026-01-20', 'http://a2', '', '', 'merged')`)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = store.db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels)
			VALUES ('org1', 'repo1', 'alice', 'issue', ?, 'http://a3', '', '')`,
			"2026-02-"+padDay(i))
		require.NoError(t, err)
	}
	// bob: 1 PR, 1 issue_comment
	_, err = store.db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'bob', 'pr', '2026-01-10', 'http://b', '', '')`)
	require.NoError(t, err)
	_, err = store.db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'bob', 'issue_comment', '2026-01-11', 'http://b2', '', '')`)
	require.NoError(t, err)

	series, err := store.GetContributorProfile("alice", nil, nil, nil, 24)
	require.NoError(t, err)
	assert.Len(t, series.Metrics, 9)
	// alice: PRs opened = 4 (3 open + 1 merged)
	assert.Equal(t, 4, series.Values[0])
	// alice: PRs merged = 1
	assert.Equal(t, 1, series.Values[1])
	// alice: Issues opened = 2
	assert.Equal(t, 2, series.Values[3])
	// alice: PR Size S = 4 (all PRs have 0 additions+deletions < 50)
	assert.Equal(t, 4, series.Values[5])
	// Averages > 0
	assert.Greater(t, series.Averages[0], float64(0))
}

func padDay(i int) string {
	return fmt.Sprintf("%02d", (i%28)+1)
}
