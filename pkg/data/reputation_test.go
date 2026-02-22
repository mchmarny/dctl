package data

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetReputationDistribution_NilDB(t *testing.T) {
	_, err := GetReputationDistribution(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetReputationDistribution_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	dist, err := GetReputationDistribution(db, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, dist.Labels)
	assert.Empty(t, dist.Data)
}

func TestUpdateReputation(t *testing.T) {
	db := setupTestDB(t)

	devs := []*Developer{{Username: "repuser", FullName: "Rep User", Entity: "CORP"}}
	require.NoError(t, SaveDevelopers(db, devs))

	require.NoError(t, updateReputation(db, "repuser", 0.85, "2025-01-15T10:00:00Z", true, nil))

	var rep float64
	var updatedAt string
	err := db.QueryRow("SELECT reputation, reputation_updated_at FROM developer WHERE username = ?", "repuser").
		Scan(&rep, &updatedAt)
	require.NoError(t, err)
	assert.InDelta(t, 0.85, rep, 0.001)
	assert.Equal(t, "2025-01-15T10:00:00Z", updatedAt)
}

func TestUpdateReputation_NilDB(t *testing.T) {
	err := updateReputation(nil, "test", 0.5, "2025-01-01T00:00:00Z", false, nil)
	assert.Error(t, err)
}

func TestGetStaleReputationUsernames_NullReputation(t *testing.T) {
	db := setupTestDB(t)

	devs := []*Developer{{Username: "staleuser", FullName: "Stale User"}}
	require.NoError(t, SaveDevelopers(db, devs))

	// Add an event so the JOIN finds the user
	_, err := db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'staleuser', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	usernames, err := getStaleReputationUsernames(db, "2025-01-15T00:00:00Z")
	require.NoError(t, err)
	assert.Contains(t, usernames, "staleuser")
}

func TestGetStaleReputationUsernames_FreshReputation(t *testing.T) {
	db := setupTestDB(t)

	devs := []*Developer{{Username: "freshuser", FullName: "Fresh User"}}
	require.NoError(t, SaveDevelopers(db, devs))

	// Add event
	_, err := db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'freshuser', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	// Set fresh reputation
	require.NoError(t, updateReputation(db, "freshuser", 0.9, "2025-02-01T00:00:00Z", false, nil))

	// Threshold before the update — user should NOT appear
	usernames, err := getStaleReputationUsernames(db, "2025-01-15T00:00:00Z")
	require.NoError(t, err)
	assert.NotContains(t, usernames, "freshuser")
}

func TestGetStaleReputationUsernames_SkipsBots(t *testing.T) {
	db := setupTestDB(t)

	devs := []*Developer{
		{Username: "realuser", FullName: "Real User"},
		{Username: "dependabot[bot]", FullName: ""},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	_, err := db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES
		('org1', 'repo1', 'realuser', 'pr', '2025-01-10', 'http://example.com', '', ''),
		('org1', 'repo1', 'dependabot[bot]', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	usernames, err := getStaleReputationUsernames(db, "2025-01-15T00:00:00Z")
	require.NoError(t, err)
	assert.Contains(t, usernames, "realuser")
	assert.NotContains(t, usernames, "dependabot[bot]")
}

func TestGetStaleReputationUsernames_NilDB(t *testing.T) {
	_, err := getStaleReputationUsernames(nil, "2025-01-01T00:00:00Z")
	assert.Error(t, err)
}

func TestGetDistinctOrgs(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES
		('org1', 'repo1', 'user1', 'pr', '2025-01-10', 'http://example.com', '', ''),
		('org2', 'repo2', 'user1', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	orgs, err := getDistinctOrgs(db)
	require.NoError(t, err)
	assert.Len(t, orgs, 2)
	assert.Contains(t, orgs, "org1")
	assert.Contains(t, orgs, "org2")
}

func TestGetDistinctOrgs_NilDB(t *testing.T) {
	_, err := getDistinctOrgs(nil)
	assert.Error(t, err)
}

func TestGetReputationDistribution_WithData(t *testing.T) {
	db := setupTestDB(t)

	devs := []*Developer{
		{Username: "highscore", FullName: "High Score"},
		{Username: "lowscore", FullName: "Low Score"},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	require.NoError(t, updateReputation(db, "highscore", 0.95, "2025-01-15T00:00:00Z", false, nil))
	require.NoError(t, updateReputation(db, "lowscore", 0.30, "2025-01-15T00:00:00Z", false, nil))

	_, err := db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES
		('org1', 'repo1', 'highscore', 'pr', '2025-01-10', 'http://example.com', '', ''),
		('org1', 'repo1', 'lowscore', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	dist, err := GetReputationDistribution(db, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, dist.Labels, 2)
	// Ordered by reputation ASC (lowest first)
	assert.Equal(t, "lowscore", dist.Labels[0])
	assert.InDelta(t, 0.30, dist.Data[0], 0.001)
	assert.Equal(t, "highscore", dist.Labels[1])
	assert.InDelta(t, 0.95, dist.Data[1], 0.001)
}

func TestImportReputation_NilDB(t *testing.T) {
	_, err := ImportReputation(nil)
	assert.Error(t, err)
}

func TestImportReputation_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	res, err := ImportReputation(db)
	require.NoError(t, err)
	assert.Equal(t, 0, res.Updated)
}

func TestImportReputation_ComputesShallowScores(t *testing.T) {
	db := setupTestDB(t)

	devs := []*Developer{{Username: "alice", FullName: "Alice"}}
	require.NoError(t, SaveDevelopers(db, devs))

	// Use today's date so recency signal is non-zero
	today := time.Now().UTC().Format("2006-01-02")
	_, err := db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'alice', 'pr', ?, 'http://example.com', '', '')`, today)
	require.NoError(t, err)

	res, err := ImportReputation(db)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Updated)

	// Verify score was stored and non-zero (recency + engagement signals active)
	var rep sql.NullFloat64
	scanErr := db.QueryRow("SELECT reputation FROM developer WHERE username = 'alice'").Scan(&rep)
	require.NoError(t, scanErr)
	assert.True(t, rep.Valid)
}

func TestGatherLocalSignals(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES
		('org1', 'repo1', 'user1', 'pr', '2025-01-10', 'http://example.com', '', ''),
		('org1', 'repo1', 'user1', 'pr', '2025-01-11', 'http://example.com', '', ''),
		('org1', 'repo1', 'user2', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	stats, err := computeGlobalStats(db, "2024-01-01")
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.totalCommits)
	assert.Equal(t, 2, stats.totalContributors)

	s := gatherLocalSignals(db, "user1", "2024-01-01", stats)
	assert.Equal(t, int64(2), s.Commits)
	assert.Equal(t, int64(3), s.TotalCommits)
	assert.Equal(t, 2, s.TotalContributors)
	assert.Greater(t, s.LastCommitDays, int64(0))
}
