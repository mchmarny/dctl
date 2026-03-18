package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRepoMetas_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	list, err := store.GetRepoMetas(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestGetRepoMetas_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetRepoMetas(nil, nil)
	assert.Error(t, err)
}

func TestGetRepoMetas_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO repo_meta (org, repo, stars, forks, open_issues, language, license, archived)
		VALUES ('org1', 'repo1', 100, 50, 10, 'Go', 'Apache-2.0', 0)`)
	require.NoError(t, err)

	list, err := store.GetRepoMetas(nil, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "org1", list[0].Org)
	assert.Equal(t, 100, list[0].Stars)
	assert.Equal(t, "Go", list[0].Language)
	assert.False(t, list[0].Archived)
}

func TestGetRepoMetas_WithFilter(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO repo_meta (org, repo, stars, forks, open_issues, language, license, archived)
		VALUES
		('org1', 'repo1', 100, 50, 10, 'Go', 'Apache-2.0', 0),
		('org2', 'repo2', 200, 80, 5, 'Python', 'MIT', 0)`)
	require.NoError(t, err)

	org := "org1"
	list, err := store.GetRepoMetas(&org, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "org1", list[0].Org)
}

func TestGetRepoOverview_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	list, err := store.GetRepoOverview(nil, 6)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestGetRepoOverview_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetRepoOverview(nil, 6)
	assert.Error(t, err)
}

func TestGetRepoOverview_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO repo_meta (org, repo, stars, forks, open_issues, language, license, archived)
		VALUES ('org1', 'repo1', 100, 50, 10, 'Go', 'Apache-2.0', 0)`)
	require.NoError(t, err)

	_, err = store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('user1', 'User One'), ('user2', 'User Two')`)
	require.NoError(t, err)

	_, err = store.db.Exec(`UPDATE developer SET reputation = 0.75 WHERE username = 'user1'`)
	require.NoError(t, err)

	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES
		('org1', 'repo1', 'user1', 'push', '2026-03-01', '', '', ''),
		('org1', 'repo1', 'user2', 'issue', '2026-03-02', '', '', '')`)
	require.NoError(t, err)

	list, err := store.GetRepoOverview(nil, 6)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "org1", list[0].Org)
	assert.Equal(t, "repo1", list[0].Repo)
	assert.Equal(t, 100, list[0].Stars)
	assert.Equal(t, 50, list[0].Forks)
	assert.Equal(t, 2, list[0].Events)
	assert.Equal(t, 2, list[0].Contributors)
	assert.Equal(t, 1, list[0].Scored)
	assert.Equal(t, "Go", list[0].Language)
	assert.Equal(t, "2026-03-02", list[0].LastImport)
}

func TestGetRepoOverview_WithOrgFilter(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO repo_meta (org, repo, stars, forks, open_issues, language, license, archived)
		VALUES
		('org1', 'repo1', 100, 50, 10, 'Go', 'Apache-2.0', 0),
		('org2', 'repo2', 200, 80, 5, 'Python', 'MIT', 0)`)
	require.NoError(t, err)

	org := "org1"
	list, err := store.GetRepoOverview(&org, 6)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "org1", list[0].Org)
}
