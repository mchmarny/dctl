package data

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanEntity(t *testing.T) {
	entityRegEx = regexp.MustCompile(nonAlphaNumRegex)

	tests := map[string]string{
		"Google LLC":           "GOOGLE",
		"Hitachi Vantara LLC":  "HITACHI VANTARA",
		"MAX KELSEN PTY. LTD.": "MAX KELSEN PTY",
		"Mercari Inc":          "MERCARI",
		"Some Company Corp.":   "SOME",
		"Big Cars LLC.":        "BIG CARS",
		"International Business Machines Corporation": "IBM",
	}

	for input, expected := range tests {
		val := cleanEntityName(input)
		assert.Equal(t, expected, val)
	}
}

func TestSaveAndGetDeveloper(t *testing.T) {
	db := setupTestDB(t)
	devs := []*Developer{
		{Username: "testuser", FullName: "Test User", Email: "test@example.com", Entity: "TESTCORP"},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	got, err := GetDeveloper(db, "testuser")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "testuser", got.Username)
	assert.Equal(t, "TESTCORP", got.Entity)
}

func TestGetDeveloper_NotFound(t *testing.T) {
	db := setupTestDB(t)
	got, err := GetDeveloper(db, "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestSearchDevelopers(t *testing.T) {
	db := setupTestDB(t)
	devs := []*Developer{
		{Username: "alice", FullName: "Alice Smith", Email: "alice@corp.com", Entity: "CORP"},
		{Username: "bob", FullName: "Bob Jones", Email: "bob@other.com", Entity: "OTHER"},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	results, err := SearchDevelopers(db, "alice", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "alice", results[0].Username)
}

func TestSaveDevelopers_Upsert(t *testing.T) {
	db := setupTestDB(t)
	devs := []*Developer{{Username: "user1", FullName: "Original", Entity: "CORP"}}
	require.NoError(t, SaveDevelopers(db, devs))

	devs[0].FullName = "Updated"
	require.NoError(t, SaveDevelopers(db, devs))

	got, err := GetDeveloper(db, "user1")
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.FullName)
}

func TestSaveDevelopers_EmptySlice(t *testing.T) {
	db := setupTestDB(t)
	assert.NoError(t, SaveDevelopers(db, []*Developer{}))
}

func TestSaveDevelopers_NilDB(t *testing.T) {
	err := SaveDevelopers(nil, []*Developer{{Username: "test"}})
	assert.Error(t, err)
}

func TestGetDeveloper_NilDB(t *testing.T) {
	_, err := GetDeveloper(nil, "test")
	assert.Error(t, err)
}

func TestSearchDevelopers_NilDB(t *testing.T) {
	_, err := SearchDevelopers(nil, "test", 10)
	assert.Error(t, err)
}

func TestGetDeveloperUsernames(t *testing.T) {
	db := setupTestDB(t)
	devs := []*Developer{
		{Username: "user1", FullName: "User One"},
		{Username: "user2", FullName: "User Two"},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	usernames, err := GetDeveloperUsernames(db)
	require.NoError(t, err)
	assert.Len(t, usernames, 2)
}

func TestUpdateDeveloperNames(t *testing.T) {
	db := setupTestDB(t)
	devs := []*Developer{
		{Username: "user1", FullName: ""},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	names := map[string]string{"user1": "Updated Name"}
	require.NoError(t, UpdateDeveloperNames(db, names))

	got, err := GetDeveloper(db, "user1")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", got.FullName)
}

func TestGetNoFullnameDeveloperUsernames(t *testing.T) {
	db := setupTestDB(t)
	devs := []*Developer{
		{Username: "withname", FullName: "Has Name"},
		{Username: "noname", FullName: ""},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	usernames, err := GetNoFullnameDeveloperUsernames(db)
	require.NoError(t, err)
	assert.Len(t, usernames, 1)
	assert.Equal(t, "noname", usernames[0])
}

func TestGetDeveloperUsernames_NilDB(t *testing.T) {
	_, err := GetDeveloperUsernames(nil)
	assert.Error(t, err)
}

func TestUpdateDeveloperNames_NilDB(t *testing.T) {
	err := UpdateDeveloperNames(nil, map[string]string{"u": "n"})
	assert.Error(t, err)
}

func TestSearchDevelopers_MultipleMatches(t *testing.T) {
	db := setupTestDB(t)
	devs := []*Developer{
		{Username: "alice1", FullName: "Alice A", Entity: "CORP"},
		{Username: "alice2", FullName: "Alice B", Entity: "CORP"},
		{Username: "bob", FullName: "Bob", Entity: "CORP"},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	results, err := SearchDevelopers(db, "alice", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}
