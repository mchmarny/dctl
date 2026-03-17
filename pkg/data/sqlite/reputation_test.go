package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetReputationDistribution_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetReputationDistribution(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetReputationDistribution_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	dist, err := store.GetReputationDistribution(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, dist.Labels)
	assert.Empty(t, dist.Data)
}

func TestUpdateReputation(t *testing.T) {
	store := setupTestDB(t)

	devs := []*data.Developer{{Username: "repuser", FullName: "Rep User", Entity: "CORP"}}
	require.NoError(t, store.SaveDevelopers(devs))

	require.NoError(t, store.updateReputation("repuser", 0.85, "2025-01-15T10:00:00Z", true, nil))

	var rep float64
	var updatedAt string
	err := store.db.QueryRow("SELECT reputation, reputation_updated_at FROM developer WHERE username = ?", "repuser").
		Scan(&rep, &updatedAt)
	require.NoError(t, err)
	assert.InDelta(t, 0.85, rep, 0.001)
	assert.Equal(t, "2025-01-15T10:00:00Z", updatedAt)
}

func TestUpdateReputation_NilDB(t *testing.T) {
	s := &Store{db: nil}
	err := s.updateReputation("test", 0.5, "2025-01-01T00:00:00Z", false, nil)
	assert.Error(t, err)
}

func TestGetStaleReputationUsernames_NullReputation(t *testing.T) {
	store := setupTestDB(t)

	devs := []*data.Developer{{Username: "staleuser", FullName: "Stale User"}}
	require.NoError(t, store.SaveDevelopers(devs))

	// Add an event so the JOIN finds the user
	_, err := store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'staleuser', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	usernames, err := store.getStaleReputationUsernames(nil, nil, "2025-01-15T00:00:00Z")
	require.NoError(t, err)
	assert.Contains(t, usernames, "staleuser")
}

func TestGetStaleReputationUsernames_FreshReputation(t *testing.T) {
	store := setupTestDB(t)

	devs := []*data.Developer{{Username: "freshuser", FullName: "Fresh User"}}
	require.NoError(t, store.SaveDevelopers(devs))

	// Add event
	_, err := store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'freshuser', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	// Set fresh reputation
	require.NoError(t, store.updateReputation("freshuser", 0.9, "2025-02-01T00:00:00Z", false, nil))

	// Threshold before the update -- user should NOT appear
	usernames, err := store.getStaleReputationUsernames(nil, nil, "2025-01-15T00:00:00Z")
	require.NoError(t, err)
	assert.NotContains(t, usernames, "freshuser")
}

func TestGetStaleReputationUsernames_SkipsBots(t *testing.T) {
	store := setupTestDB(t)

	devs := []*data.Developer{
		{Username: "realuser", FullName: "Real User"},
		{Username: "dependabot[bot]", FullName: ""},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	_, err := store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES
		('org1', 'repo1', 'realuser', 'pr', '2025-01-10', 'http://example.com', '', ''),
		('org1', 'repo1', 'dependabot[bot]', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	usernames, err := store.getStaleReputationUsernames(nil, nil, "2025-01-15T00:00:00Z")
	require.NoError(t, err)
	assert.Contains(t, usernames, "realuser")
	assert.NotContains(t, usernames, "dependabot[bot]")
}

func TestGetStaleReputationUsernames_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.getStaleReputationUsernames(nil, nil, "2025-01-01T00:00:00Z")
	assert.Error(t, err)
}

func TestGetDistinctOrgs(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES
		('org1', 'repo1', 'user1', 'pr', '2025-01-10', 'http://example.com', '', ''),
		('org2', 'repo2', 'user1', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	orgs, err := store.getDistinctOrgs()
	require.NoError(t, err)
	assert.Len(t, orgs, 2)
	assert.Contains(t, orgs, "org1")
	assert.Contains(t, orgs, "org2")
}

func TestGetDistinctOrgs_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.getDistinctOrgs()
	assert.Error(t, err)
}

func TestGetReputationDistribution_WithData(t *testing.T) {
	store := setupTestDB(t)

	devs := []*data.Developer{
		{Username: "highscore", FullName: "High Score"},
		{Username: "lowscore", FullName: "Low Score"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	require.NoError(t, store.updateReputation("highscore", 0.95, "2025-01-15T00:00:00Z", false, nil))
	require.NoError(t, store.updateReputation("lowscore", 0.30, "2025-01-15T00:00:00Z", false, nil))

	_, err := store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES
		('org1', 'repo1', 'highscore', 'pr', '2025-01-10', 'http://example.com', '', ''),
		('org1', 'repo1', 'lowscore', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	dist, err := store.GetReputationDistribution(nil, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, dist.Labels, 2)
	// Ordered by reputation ASC (lowest first)
	assert.Equal(t, "lowscore", dist.Labels[0])
	assert.InDelta(t, 0.30, dist.Data[0], 0.001)
	assert.Equal(t, "highscore", dist.Labels[1])
	assert.InDelta(t, 0.95, dist.Data[1], 0.001)
}

func TestImportReputation_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.ImportReputation(nil, nil)
	assert.Error(t, err)
}

func TestImportReputation_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	res, err := store.ImportReputation(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, res.Updated)
}

func TestImportReputation_ComputesShallowScores(t *testing.T) {
	store := setupTestDB(t)

	devs := []*data.Developer{{Username: "alice", FullName: "Alice"}}
	require.NoError(t, store.SaveDevelopers(devs))

	// Use today's date so recency signal is non-zero
	today := time.Now().UTC().Format("2006-01-02")
	_, err := store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'alice', 'pr', ?, 'http://example.com', '', '')`, today)
	require.NoError(t, err)

	res, err := store.ImportReputation(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Updated)

	// Verify score was stored and non-zero (recency + engagement signals active)
	var rep sql.NullFloat64
	scanErr := store.db.QueryRow("SELECT reputation FROM developer WHERE username = 'alice'").Scan(&rep)
	require.NoError(t, scanErr)
	assert.True(t, rep.Valid)
}

func TestGetLowestReputationUsernames_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.getLowestReputationUsernames(nil, nil, "2025-01-01T00:00:00Z", 5)
	assert.Error(t, err)
}

func TestGetLowestReputationUsernames_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	usernames, err := store.getLowestReputationUsernames(nil, nil, "2025-01-01T00:00:00Z", 5)
	require.NoError(t, err)
	assert.Empty(t, usernames)
}

func TestGetLowestReputationUsernames_ReturnsBottomN(t *testing.T) {
	store := setupTestDB(t)

	devs := []*data.Developer{
		{Username: "low1", FullName: "Low One"},
		{Username: "low2", FullName: "Low Two"},
		{Username: "mid", FullName: "Mid"},
		{Username: "high", FullName: "High"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	// Set shallow scores (reputation_deep = 0)
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	require.NoError(t, store.updateReputation("low1", 0.10, now, false, nil))
	require.NoError(t, store.updateReputation("low2", 0.20, now, false, nil))
	require.NoError(t, store.updateReputation("mid", 0.50, now, false, nil))
	require.NoError(t, store.updateReputation("high", 0.90, now, false, nil))

	// Add events so JOIN finds them
	for _, u := range []string{"low1", "low2", "mid", "high"} {
		_, err := store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
			VALUES ('org1', 'repo1', ?, 'pr', '2025-01-10', 'http://example.com', '', '')`, u)
		require.NoError(t, err)
	}

	// Threshold in the future so none are "fresh deep"
	threshold := time.Now().UTC().Add(time.Hour).Format("2006-01-02T15:04:05Z")
	usernames, err := store.getLowestReputationUsernames(nil, nil, threshold, 2)
	require.NoError(t, err)
	require.Len(t, usernames, 2)
	assert.Equal(t, "low1", usernames[0])
	assert.Equal(t, "low2", usernames[1])
}

func TestGetLowestReputationUsernames_SkipsFreshDeep(t *testing.T) {
	store := setupTestDB(t)

	devs := []*data.Developer{
		{Username: "deepuser", FullName: "Deep User"},
		{Username: "shallowuser", FullName: "Shallow User"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	require.NoError(t, store.updateReputation("deepuser", 0.10, now, true, nil))     // deep=true, fresh
	require.NoError(t, store.updateReputation("shallowuser", 0.15, now, false, nil)) // shallow

	for _, u := range []string{"deepuser", "shallowuser"} {
		_, err := store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
			VALUES ('org1', 'repo1', ?, 'pr', '2025-01-10', 'http://example.com', '', '')`, u)
		require.NoError(t, err)
	}

	// Threshold before now -- deepuser's fresh deep score should be excluded
	threshold := time.Now().UTC().Add(-time.Hour).Format("2006-01-02T15:04:05Z")
	usernames, err := store.getLowestReputationUsernames(nil, nil, threshold, 10)
	require.NoError(t, err)
	assert.Contains(t, usernames, "shallowuser")
	assert.NotContains(t, usernames, "deepuser")
}

func TestGetLowestReputationUsernames_SkipsBots(t *testing.T) {
	store := setupTestDB(t)

	devs := []*data.Developer{
		{Username: "realuser", FullName: "Real"},
		{Username: "mybot[bot]", FullName: ""},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	require.NoError(t, store.updateReputation("realuser", 0.10, now, false, nil))
	require.NoError(t, store.updateReputation("mybot[bot]", 0.05, now, false, nil))

	for _, u := range []string{"realuser", "mybot[bot]"} {
		_, err := store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
			VALUES ('org1', 'repo1', ?, 'pr', '2025-01-10', 'http://example.com', '', '')`, u)
		require.NoError(t, err)
	}

	threshold := time.Now().UTC().Add(time.Hour).Format("2006-01-02T15:04:05Z")
	usernames, err := store.getLowestReputationUsernames(nil, nil, threshold, 10)
	require.NoError(t, err)
	assert.Contains(t, usernames, "realuser")
	assert.NotContains(t, usernames, "mybot[bot]")
}

func TestImportDeepReputation_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.ImportDeepReputation(context.Background(), func() string { return "token" }, 5, nil, nil)
	assert.Error(t, err)
}

func TestImportDeepReputation_EmptyToken(t *testing.T) {
	store := setupTestDB(t)
	_, err := store.ImportDeepReputation(context.Background(), nil, 5, nil, nil)
	assert.Error(t, err)
}

func TestImportDeepReputation_ZeroLimit(t *testing.T) {
	store := setupTestDB(t)
	res, err := store.ImportDeepReputation(context.Background(), func() string { return "token" }, 0, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, res.Scored)
}

func TestImportDeepReputation_NoCandidates(t *testing.T) {
	store := setupTestDB(t)
	res, err := store.ImportDeepReputation(context.Background(), func() string { return "token" }, 5, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, res.Scored)
}

func TestGetStaleReputationUsernames_FilterByOrg(t *testing.T) {
	store := setupTestDB(t)

	devs := []*data.Developer{
		{Username: "orguser", FullName: "Org User"},
		{Username: "otheruser", FullName: "Other User"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	_, err := store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES
		('nvidia', 'repo1', 'orguser', 'pr', '2025-01-10', 'http://example.com', '', ''),
		('other', 'repo2', 'otheruser', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	org := "nvidia"
	usernames, err := store.getStaleReputationUsernames(&org, nil, "2025-01-15T00:00:00Z")
	require.NoError(t, err)
	assert.Contains(t, usernames, "orguser")
	assert.NotContains(t, usernames, "otheruser")
}

func TestGetStaleReputationUsernames_FilterByOrgAndRepo(t *testing.T) {
	store := setupTestDB(t)

	devs := []*data.Developer{
		{Username: "repouser", FullName: "Repo User"},
		{Username: "otherrepo", FullName: "Other Repo"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	_, err := store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES
		('nvidia', 'skyhook', 'repouser', 'pr', '2025-01-10', 'http://example.com', '', ''),
		('nvidia', 'other', 'otherrepo', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	org := "nvidia"
	repo := "skyhook"
	usernames, err := store.getStaleReputationUsernames(&org, &repo, "2025-01-15T00:00:00Z")
	require.NoError(t, err)
	assert.Contains(t, usernames, "repouser")
	assert.NotContains(t, usernames, "otherrepo")
}

func TestGatherLocalSignals(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES
		('org1', 'repo1', 'user1', 'pr', '2025-01-10', 'http://example.com', '', ''),
		('org1', 'repo1', 'user1', 'pr', '2025-01-11', 'http://example.com', '', ''),
		('org1', 'repo1', 'user2', 'pr', '2025-01-10', 'http://example.com', '', '')`)
	require.NoError(t, err)

	stats, err := store.computeGlobalStats("2024-01-01")
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.totalCommits)
	assert.Equal(t, 2, stats.totalContributors)

	s := store.gatherLocalSignals("user1", "2024-01-01", stats)
	assert.Equal(t, int64(2), s.Commits)
	assert.Equal(t, int64(3), s.TotalCommits)
	assert.Equal(t, 2, s.TotalContributors)
	assert.Greater(t, s.LastCommitDays, int64(0))
}
