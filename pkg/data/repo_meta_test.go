package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRepoMetas_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	list, err := GetRepoMetas(db, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestGetRepoMetas_NilDB(t *testing.T) {
	_, err := GetRepoMetas(nil, nil, nil)
	assert.Error(t, err)
}

func TestGetRepoMetas_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO repo_meta (org, repo, stars, forks, open_issues, language, license, archived)
		VALUES ('org1', 'repo1', 100, 50, 10, 'Go', 'Apache-2.0', 0)`)
	require.NoError(t, err)

	list, err := GetRepoMetas(db, nil, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "org1", list[0].Org)
	assert.Equal(t, 100, list[0].Stars)
	assert.Equal(t, "Go", list[0].Language)
	assert.False(t, list[0].Archived)
}

func TestGetRepoMetas_WithFilter(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO repo_meta (org, repo, stars, forks, open_issues, language, license, archived)
		VALUES
		('org1', 'repo1', 100, 50, 10, 'Go', 'Apache-2.0', 0),
		('org2', 'repo2', 200, 80, 5, 'Python', 'MIT', 0)`)
	require.NoError(t, err)

	org := "org1"
	list, err := GetRepoMetas(db, &org, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "org1", list[0].Org)
}
