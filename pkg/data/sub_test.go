package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndApplyDeveloperSub(t *testing.T) {
	db := setupTestDB(t)
	devs := []*Developer{
		{Username: "user1", FullName: "User One", Entity: "OLDNAME"},
		{Username: "user2", FullName: "User Two", Entity: "OLDNAME"},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	sub, err := SaveAndApplyDeveloperSub(db, "entity", "OLDNAME", "NEWNAME")
	require.NoError(t, err)
	assert.Equal(t, int64(2), sub.Records)

	dev, err := GetDeveloper(db, "user1")
	require.NoError(t, err)
	assert.Equal(t, "NEWNAME", dev.Entity)
}

func TestSaveAndApplyDeveloperSub_InvalidProperty(t *testing.T) {
	db := setupTestDB(t)
	_, err := SaveAndApplyDeveloperSub(db, "invalid_prop", "old", "new")
	assert.Error(t, err)
}

func TestSaveAndApplyDeveloperSub_NilDB(t *testing.T) {
	_, err := SaveAndApplyDeveloperSub(nil, "entity", "old", "new")
	assert.Error(t, err)
}

func TestApplySubstitutions(t *testing.T) {
	db := setupTestDB(t)
	devs := []*Developer{
		{Username: "u1", FullName: "U1", Entity: "OLD"},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	_, err := SaveAndApplyDeveloperSub(db, "entity", "OLD", "NEW")
	require.NoError(t, err)

	// Change entity back to OLD to verify re-application
	devs[0].Entity = "OLD"
	require.NoError(t, SaveDevelopers(db, devs))

	subs, err := ApplySubstitutions(db)
	require.NoError(t, err)
	assert.NotEmpty(t, subs)

	dev, err := GetDeveloper(db, "u1")
	require.NoError(t, err)
	assert.Equal(t, "NEW", dev.Entity)
}

func TestApplySubstitutions_NilDB(t *testing.T) {
	_, err := ApplySubstitutions(nil)
	assert.Error(t, err)
}
