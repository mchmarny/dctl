package sqlite

import (
	"testing"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndApplyDeveloperSub(t *testing.T) {
	store := setupTestDB(t)
	devs := []*data.Developer{
		{Username: "user1", FullName: "User One", Entity: "OLDNAME"},
		{Username: "user2", FullName: "User Two", Entity: "OLDNAME"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	sub, err := store.SaveAndApplyDeveloperSub("entity", "OLDNAME", "NEWNAME")
	require.NoError(t, err)
	assert.Equal(t, int64(2), sub.Records)

	dev, err := store.GetDeveloper("user1")
	require.NoError(t, err)
	assert.Equal(t, "NEWNAME", dev.Entity)
}

func TestSaveAndApplyDeveloperSub_InvalidProperty(t *testing.T) {
	store := setupTestDB(t)
	_, err := store.SaveAndApplyDeveloperSub("invalid_prop", "old", "new")
	assert.Error(t, err)
}

func TestSaveAndApplyDeveloperSub_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.SaveAndApplyDeveloperSub("entity", "old", "new")
	assert.Error(t, err)
}

func TestApplySubstitutions(t *testing.T) {
	store := setupTestDB(t)
	devs := []*data.Developer{
		{Username: "u1", FullName: "U1", Entity: "OLD"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	_, err := store.SaveAndApplyDeveloperSub("entity", "OLD", "NEW")
	require.NoError(t, err)

	// Change entity back to OLD to verify re-application
	devs[0].Entity = "OLD"
	require.NoError(t, store.SaveDevelopers(devs))

	subs, err := store.ApplySubstitutions()
	require.NoError(t, err)
	assert.NotEmpty(t, subs)

	dev, err := store.GetDeveloper("u1")
	require.NoError(t, err)
	assert.Equal(t, "NEW", dev.Entity)
}

func TestApplySubstitutions_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.ApplySubstitutions()
	assert.Error(t, err)
}
