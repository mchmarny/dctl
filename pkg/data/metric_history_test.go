package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRepoMetricHistory_NilDB(t *testing.T) {
	_, err := GetRepoMetricHistory(nil, nil, nil)
	assert.Error(t, err)
}

func TestGetRepoMetricHistory_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	list, err := GetRepoMetricHistory(db, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestGetRepoMetricHistory_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO repo_metric_history (org, repo, date, stars, forks)
		VALUES
		('org1', 'repo1', '2026-03-10', 100, 50),
		('org1', 'repo1', '2026-03-11', 105, 52),
		('org1', 'repo1', '2026-03-12', 110, 55)`)
	require.NoError(t, err)

	list, err := GetRepoMetricHistory(db, nil, nil)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "2026-03-10", list[0].Date)
	assert.Equal(t, 110, list[2].Stars)
	assert.Equal(t, 55, list[2].Forks)
}

func TestGetRepoMetricHistory_WithFilter(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO repo_metric_history (org, repo, date, stars, forks)
		VALUES
		('org1', 'repo1', '2026-03-10', 100, 50),
		('org2', 'repo2', '2026-03-10', 200, 80)`)
	require.NoError(t, err)

	org := "org1"
	list, err := GetRepoMetricHistory(db, &org, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "org1", list[0].Org)
}
