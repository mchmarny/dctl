package sqlite

import (
	"testing"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/devpulse/pkg/data/ghutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllOrgRepos(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	repos, err := store.GetAllOrgRepos()
	require.NoError(t, err)
	assert.NotEmpty(t, repos)
}

func TestGetAllOrgRepos_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetAllOrgRepos()
	assert.Error(t, err)
}

func TestGetOrgLike(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	items, err := store.GetOrgLike("test", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, items)
}

func TestGetOrgLike_EmptyQuery(t *testing.T) {
	store := setupTestDB(t)
	_, err := store.GetOrgLike("", 10)
	assert.Error(t, err)
}

func TestGetOrgLike_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetOrgLike("test", 10)
	assert.Error(t, err)
}

func TestGetAllOrgRepos_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	repos, err := store.GetAllOrgRepos()
	require.NoError(t, err)
	assert.Empty(t, repos)
}

func TestMapOrg(t *testing.T) {
	login := "testorg"
	company := "TestCo"
	desc := "An org"
	url := "https://github.com/testorg"
	o := &github.Organization{
		Login:       &login,
		Company:     &company,
		Description: &desc,
		URL:         &url,
	}
	org := ghutil.MapOrg(o)
	assert.Equal(t, "testorg", org.Name)
	assert.Equal(t, "TestCo", org.Company)
	assert.Equal(t, "An org", org.Description)
}

func TestGetDeveloperPercentages_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetDeveloperPercentages(nil, nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetEntityPercentages_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetEntityPercentages(nil, nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetDeveloperPercentages(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	results, err := store.GetDeveloperPercentages(nil, nil, nil, []string{}, 12)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestGetEntityPercentages(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	results, err := store.GetEntityPercentages(nil, nil, nil, []string{}, 12)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestSearchDeveloperUsernames_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.SearchDeveloperUsernames("dev", nil, nil, 6, 10)
	assert.Error(t, err)
}

func TestSearchDeveloperUsernames_EmptyQuery(t *testing.T) {
	store := setupTestDB(t)
	_, err := store.SearchDeveloperUsernames("", nil, nil, 6, 10)
	assert.Error(t, err)
}

func TestSearchDeveloperUsernames_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	results, err := store.SearchDeveloperUsernames("dev", nil, nil, 6, 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchDeveloperUsernames_WithData(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	results, err := store.SearchDeveloperUsernames("dev", nil, nil, 12, 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	for _, r := range results {
		assert.Contains(t, r, "dev")
	}
}
