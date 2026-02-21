package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetState_NoExistingState(t *testing.T) {
	db := setupTestDB(t)
	min := time.Now().AddDate(0, -6, 0).UTC()
	state, err := GetState(db, "pr", "testorg", "testrepo", min)
	require.NoError(t, err)
	assert.Equal(t, 1, state.Page)
}

func TestSaveAndGetState(t *testing.T) {
	db := setupTestDB(t)
	min := time.Now().AddDate(0, -6, 0).UTC()
	since := time.Now().AddDate(0, -3, 0).UTC()

	s := &State{Page: 5, Since: since}
	err := SaveState(db, "pr", "testorg", "testrepo", s)
	require.NoError(t, err)

	got, err := GetState(db, "pr", "testorg", "testrepo", min)
	require.NoError(t, err)
	assert.Equal(t, 5, got.Page)
}

func TestSaveState_NilState(t *testing.T) {
	db := setupTestDB(t)
	err := SaveState(db, "pr", "org", "repo", nil)
	assert.Error(t, err)
}

func TestSaveState_EmptyParams(t *testing.T) {
	db := setupTestDB(t)
	s := &State{Page: 1, Since: time.Now()}
	err := SaveState(db, "", "org", "repo", s)
	assert.Error(t, err)
}

func TestGetDataState_NilDB(t *testing.T) {
	_, err := GetDataState(nil)
	assert.Error(t, err)
}

func TestSaveState_Upsert(t *testing.T) {
	db := setupTestDB(t)
	since := time.Now().AddDate(0, -3, 0).UTC()

	s := &State{Page: 1, Since: since}
	require.NoError(t, SaveState(db, "pr", "testorg", "testrepo", s))

	// Saving again with same key should not error (upsert)
	s.Page = 10
	assert.NoError(t, SaveState(db, "pr", "testorg", "testrepo", s))
}

func TestGetState_NilDB(t *testing.T) {
	min := time.Now().AddDate(0, -6, 0).UTC()
	_, err := GetState(nil, "pr", "org", "repo", min)
	assert.Error(t, err)
}

func TestSaveState_NilDB(t *testing.T) {
	s := &State{Page: 1, Since: time.Now()}
	err := SaveState(nil, "pr", "org", "repo", s)
	assert.Error(t, err)
}
