package data

import (
	"testing"

	"github.com/google/go-github/v83/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRepoLike(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	items, err := GetRepoLike(db, "test", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, items)
	assert.Contains(t, items[0].Value, "testorg/testrepo")
}

func TestGetRepoLike_EmptyQuery(t *testing.T) {
	db := setupTestDB(t)
	_, err := GetRepoLike(db, "", 10)
	assert.Error(t, err)
}

func TestGetRepoLike_NilDB(t *testing.T) {
	_, err := GetRepoLike(nil, "test", 10)
	assert.Error(t, err)
}

func TestGetRepoLike_NoResults(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	items, err := GetRepoLike(db, "nonexistent", 10)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestMapRepo(t *testing.T) {
	name := "my-repo"
	fullName := "org/my-repo"
	desc := "A test repo"
	htmlURL := "https://github.com/org/my-repo"
	r := &github.Repository{
		Name:        &name,
		FullName:    &fullName,
		Description: &desc,
		HTMLURL:     &htmlURL,
	}
	repo := mapRepo(r)
	assert.Equal(t, "my-repo", repo.Name)
	assert.Equal(t, "org/my-repo", repo.FullName)
	assert.Equal(t, "A test repo", repo.Description)
	assert.Equal(t, "https://github.com/org/my-repo", repo.URL)
}
