package data

import (
	"testing"

	"github.com/google/go-github/v83/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllOrgRepos(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	repos, err := GetAllOrgRepos(db)
	require.NoError(t, err)
	assert.NotEmpty(t, repos)
}

func TestGetAllOrgRepos_NilDB(t *testing.T) {
	_, err := GetAllOrgRepos(nil)
	assert.Error(t, err)
}

func TestGetOrgLike(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	items, err := GetOrgLike(db, "test", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, items)
}

func TestGetOrgLike_EmptyQuery(t *testing.T) {
	db := setupTestDB(t)
	_, err := GetOrgLike(db, "", 10)
	assert.Error(t, err)
}

func TestGetOrgLike_NilDB(t *testing.T) {
	_, err := GetOrgLike(nil, "test", 10)
	assert.Error(t, err)
}

func TestGetAllOrgRepos_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	repos, err := GetAllOrgRepos(db)
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
	org := mapOrg(o)
	assert.Equal(t, "testorg", org.Name)
	assert.Equal(t, "TestCo", org.Company)
	assert.Equal(t, "An org", org.Description)
}

func TestGetDeveloperPercentages_NilDB(t *testing.T) {
	_, err := GetDeveloperPercentages(nil, nil, nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetEntityPercentages_NilDB(t *testing.T) {
	_, err := GetEntityPercentages(nil, nil, nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetDeveloperPercentages(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	results, err := GetDeveloperPercentages(db, nil, nil, nil, []string{}, 12)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestGetEntityPercentages(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	results, err := GetEntityPercentages(db, nil, nil, nil, []string{}, 12)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}
