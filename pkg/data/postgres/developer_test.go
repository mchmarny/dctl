package postgres

import (
	"testing"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndGetDeveloper(t *testing.T) {
	store := setupTestDB(t)
	devs := []*data.Developer{
		{Username: "testuser", FullName: "Test User", Email: "test@example.com", Entity: "TESTCORP"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	got, err := store.GetDeveloper("testuser")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "testuser", got.Username)
	assert.Equal(t, "TESTCORP", got.Entity)
}

func TestGetDeveloper_NotFound(t *testing.T) {
	store := setupTestDB(t)
	got, err := store.GetDeveloper("nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestSearchDevelopers(t *testing.T) {
	store := setupTestDB(t)
	devs := []*data.Developer{
		{Username: "alice", FullName: "Alice Smith", Email: "alice@corp.com", Entity: "CORP"},
		{Username: "bob", FullName: "Bob Jones", Email: "bob@other.com", Entity: "OTHER"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	results, err := store.SearchDevelopers("alice", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "alice", results[0].Username)
}

func TestSaveDevelopers_Upsert(t *testing.T) {
	store := setupTestDB(t)
	devs := []*data.Developer{{Username: "user1", FullName: "Original", Entity: "CORP"}}
	require.NoError(t, store.SaveDevelopers(devs))

	devs[0].FullName = "Updated"
	require.NoError(t, store.SaveDevelopers(devs))

	got, err := store.GetDeveloper("user1")
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.FullName)
}

func TestSaveDevelopers_EmptySlice(t *testing.T) {
	store := setupTestDB(t)
	assert.NoError(t, store.SaveDevelopers([]*data.Developer{}))
}

func TestSaveDevelopers_NilDB(t *testing.T) {
	s := &Store{db: nil}
	err := s.SaveDevelopers([]*data.Developer{{Username: "test"}})
	assert.Error(t, err)
}

func TestGetDeveloper_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetDeveloper("test")
	assert.Error(t, err)
}

func TestSearchDevelopers_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.SearchDevelopers("test", 10)
	assert.Error(t, err)
}

func TestGetDeveloperUsernames(t *testing.T) {
	store := setupTestDB(t)
	devs := []*data.Developer{
		{Username: "user1", FullName: "User One"},
		{Username: "user2", FullName: "User Two"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	usernames, err := store.GetDeveloperUsernames()
	require.NoError(t, err)
	assert.Len(t, usernames, 2)
}

func TestUpdateDeveloperNames(t *testing.T) {
	store := setupTestDB(t)
	devs := []*data.Developer{
		{Username: "user1", FullName: ""},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	names := map[string]string{"user1": "Updated Name"}
	require.NoError(t, store.UpdateDeveloperNames(names))

	got, err := store.GetDeveloper("user1")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", got.FullName)
}

func TestGetNoFullnameDeveloperUsernames(t *testing.T) {
	store := setupTestDB(t)
	devs := []*data.Developer{
		{Username: "withname", FullName: "Has Name"},
		{Username: "noname", FullName: ""},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	usernames, err := store.GetNoFullnameDeveloperUsernames()
	require.NoError(t, err)
	assert.Len(t, usernames, 1)
	assert.Equal(t, "noname", usernames[0])
}

func TestGetDeveloperUsernames_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetDeveloperUsernames()
	assert.Error(t, err)
}

func TestUpdateDeveloperNames_NilDB(t *testing.T) {
	s := &Store{db: nil}
	err := s.UpdateDeveloperNames(map[string]string{"u": "n"})
	assert.Error(t, err)
}

func TestSearchDevelopers_MultipleMatches(t *testing.T) {
	store := setupTestDB(t)
	devs := []*data.Developer{
		{Username: "alice1", FullName: "Alice A", Entity: "CORP"},
		{Username: "alice2", FullName: "Alice B", Entity: "CORP"},
		{Username: "bob", FullName: "Bob", Entity: "CORP"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	results, err := store.SearchDevelopers("alice", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}
