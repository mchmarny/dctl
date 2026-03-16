package sqlite

import (
	"testing"
	"time"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRepoMetricHistory_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetRepoMetricHistory(nil, nil)
	assert.Error(t, err)
}

func TestGetRepoMetricHistory_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	list, err := store.GetRepoMetricHistory(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestGetRepoMetricHistory_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO repo_metric_history (org, repo, date, stars, forks)
		VALUES
		('org1', 'repo1', '2026-03-10', 100, 50),
		('org1', 'repo1', '2026-03-11', 105, 52),
		('org1', 'repo1', '2026-03-12', 110, 55)`)
	require.NoError(t, err)

	list, err := store.GetRepoMetricHistory(nil, nil)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "2026-03-10", list[0].Date)
	assert.Equal(t, 110, list[2].Stars)
	assert.Equal(t, 55, list[2].Forks)
}

func TestGetRepoMetricHistory_WithFilter(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO repo_metric_history (org, repo, date, stars, forks)
		VALUES
		('org1', 'repo1', '2026-03-10', 100, 50),
		('org2', 'repo2', '2026-03-10', 200, 80)`)
	require.NoError(t, err)

	org := "org1"
	list, err := store.GetRepoMetricHistory(&org, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "org1", list[0].Org)
}

func TestBuildDailyTotals(t *testing.T) {
	starsByDay := map[string]int{
		time.Now().UTC().Format("2006-01-02"):                   5,
		time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02"): 3,
	}
	forksByDay := map[string]int{
		time.Now().UTC().Format("2006-01-02"): 2,
	}

	result := buildDailyTotals(100, 50, starsByDay, forksByDay, 3)

	require.Len(t, result, 4) // days+1
	assert.Equal(t, 100, result[3].Stars)
	assert.Equal(t, 50, result[3].Forks)
	assert.Equal(t, 95, result[2].Stars)
	assert.Equal(t, 48, result[2].Forks)
	assert.Equal(t, 92, result[1].Stars)
}

func TestUpsertMetricHistory(t *testing.T) {
	store := setupTestDB(t)

	history := []*data.RepoMetricHistory{
		{Date: "2026-03-10", Stars: 100, Forks: 50},
		{Date: "2026-03-11", Stars: 105, Forks: 52},
	}

	err := store.upsertMetricHistory("org1", "repo1", history)
	require.NoError(t, err)

	list, err := store.GetRepoMetricHistory(nil, nil)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, 100, list[0].Stars)

	// Upsert same dates with new values.
	history[0].Stars = 101
	err = store.upsertMetricHistory("org1", "repo1", history)
	require.NoError(t, err)

	list, err = store.GetRepoMetricHistory(nil, nil)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, 101, list[0].Stars)
}

func TestBuildDailyTotals_FloorAtZero(t *testing.T) {
	starsByDay := map[string]int{
		time.Now().UTC().Format("2006-01-02"): 999,
	}
	result := buildDailyTotals(10, 5, starsByDay, nil, 1)
	require.Len(t, result, 2)
	assert.Equal(t, 10, result[1].Stars)
	assert.Equal(t, 0, result[0].Stars) // floored at zero
}
