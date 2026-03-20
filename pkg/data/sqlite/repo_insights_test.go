package sqlite

import (
	"testing"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRepoInsights_NilDB(t *testing.T) {
	s := &Store{}
	_, err := s.GetRepoInsights(nil, nil)
	require.ErrorIs(t, err, data.ErrDBNotInitialized)
}

func TestGetRepoInsights_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	list, err := store.GetRepoInsights(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestSaveAndGetRepoInsights(t *testing.T) {
	store := setupTestDB(t)
	org, repo := "org1", "repo1"
	ri := &data.RepoInsights{
		Insights: &data.GeneratedInsights{
			Observations: []data.InsightBullet{{Headline: "Test obs", Detail: "detail"}},
			Actions:      []data.InsightBullet{{Headline: "Test act", Detail: "do it"}},
		},
		PeriodMonths: 3,
		Model:        "claude-haiku-4-5-20251001",
		GeneratedAt:  "2025-01-15T00:00:00Z",
	}
	err := store.SaveRepoInsights(org, repo, ri)
	require.NoError(t, err)

	list, err := store.GetRepoInsights(&org, &repo)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "org1", list[0].Org)
	assert.Equal(t, "Test obs", list[0].Insights.Observations[0].Headline)
	assert.Equal(t, "claude-haiku-4-5-20251001", list[0].Model)
}

func TestGetRepoInsightsGeneratedAt(t *testing.T) {
	store := setupTestDB(t)
	ts, err := store.GetRepoInsightsGeneratedAt("org1", "repo1")
	require.NoError(t, err)
	assert.Empty(t, ts)

	ri := &data.RepoInsights{
		Insights:     &data.GeneratedInsights{},
		PeriodMonths: 3,
		Model:        "test",
		GeneratedAt:  "2025-01-15T00:00:00Z",
	}
	require.NoError(t, store.SaveRepoInsights("org1", "repo1", ri))

	ts, err = store.GetRepoInsightsGeneratedAt("org1", "repo1")
	require.NoError(t, err)
	assert.Equal(t, "2025-01-15T00:00:00Z", ts)
}
