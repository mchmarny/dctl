package postgres

import (
	"testing"
	"time"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetState_NoExistingState(t *testing.T) {
	store := setupTestDB(t)
	min := time.Now().AddDate(0, -6, 0).UTC()
	state, err := store.GetState("pr", "testorg", "testrepo", min)
	require.NoError(t, err)
	assert.Equal(t, 1, state.Page)
}

func TestSaveAndGetState(t *testing.T) {
	store := setupTestDB(t)
	min := time.Now().AddDate(0, -6, 0).UTC()
	since := time.Now().AddDate(0, -3, 0).UTC()

	s := &data.State{Page: 5, Since: since}
	err := store.SaveState("pr", "testorg", "testrepo", s)
	require.NoError(t, err)

	got, err := store.GetState("pr", "testorg", "testrepo", min)
	require.NoError(t, err)
	assert.Equal(t, 5, got.Page)
}

func TestSaveState_NilState(t *testing.T) {
	store := setupTestDB(t)
	err := store.SaveState("pr", "org", "repo", nil)
	assert.Error(t, err)
}

func TestSaveState_EmptyParams(t *testing.T) {
	store := setupTestDB(t)
	s := &data.State{Page: 1, Since: time.Now()}
	err := store.SaveState("", "org", "repo", s)
	assert.Error(t, err)
}

func TestGetDataState_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetDataState()
	assert.Error(t, err)
}

func TestSaveState_Upsert(t *testing.T) {
	store := setupTestDB(t)
	since := time.Now().AddDate(0, -3, 0).UTC()

	s := &data.State{Page: 1, Since: since}
	require.NoError(t, store.SaveState("pr", "testorg", "testrepo", s))

	// Saving again with same key should not error (upsert)
	s.Page = 10
	assert.NoError(t, store.SaveState("pr", "testorg", "testrepo", s))
}

func TestGetState_NilDB(t *testing.T) {
	s := &Store{db: nil}
	min := time.Now().AddDate(0, -6, 0).UTC()
	_, err := s.GetState("pr", "org", "repo", min)
	assert.Error(t, err)
}

func TestSaveState_NilDB(t *testing.T) {
	s := &Store{db: nil}
	st := &data.State{Page: 1, Since: time.Now()}
	err := s.SaveState("pr", "org", "repo", st)
	assert.Error(t, err)
}
