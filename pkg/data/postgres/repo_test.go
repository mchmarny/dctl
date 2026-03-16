package postgres

import (
	"testing"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/devpulse/pkg/data/ghutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRepoLike(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	items, err := store.GetRepoLike("test", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, items)
	assert.Contains(t, items[0].Value, "testorg/testrepo")
}

func TestGetRepoLike_EmptyQuery(t *testing.T) {
	store := setupTestDB(t)
	_, err := store.GetRepoLike("", 10)
	assert.Error(t, err)
}

func TestGetRepoLike_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetRepoLike("test", 10)
	assert.Error(t, err)
}

func TestGetRepoLike_NoResults(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	items, err := store.GetRepoLike("nonexistent", 10)
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
	repo := ghutil.MapRepo(r)
	assert.Equal(t, "my-repo", repo.Name)
	assert.Equal(t, "org/my-repo", repo.FullName)
	assert.Equal(t, "A test repo", repo.Description)
	assert.Equal(t, "https://github.com/org/my-repo", repo.URL)
}
