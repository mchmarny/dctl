package data

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedTestData(t *testing.T, db *sql.DB) {
	t.Helper()
	devs := []*Developer{
		{Username: "dev1", FullName: "Dev One", Email: "dev1@google.com", Entity: "GOOGLE"},
		{Username: "dev2", FullName: "Dev Two", Email: "dev2@google.com", Entity: "GOOGLE"},
		{Username: "dev3", FullName: "Dev Three", Email: "dev3@msft.com", Entity: "MICROSOFT"},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	tx, err := db.Begin()
	require.NoError(t, err)
	stmt, err := db.Prepare(insertEventSQL)
	require.NoError(t, err)

	events := []Event{
		{Org: "testorg", Repo: "testrepo", Username: "dev1", Type: EventTypePR, Date: "2026-01-15", URL: "https://github.com/pr/1", Mentions: "", Labels: ""},
		{Org: "testorg", Repo: "testrepo", Username: "dev2", Type: EventTypeIssue, Date: "2026-01-16", URL: "https://github.com/issue/1", Mentions: "dev1", Labels: "bug"},
		{Org: "testorg", Repo: "testrepo", Username: "dev3", Type: EventTypePR, Date: "2026-01-17", URL: "https://github.com/pr/2", Mentions: "", Labels: ""},
	}

	for _, e := range events {
		_, err = tx.Stmt(stmt).Exec(
			e.Org, e.Repo, e.Username, e.Type, e.Date,
			e.URL, e.Mentions, e.Labels,
			e.State, e.Number, e.CreatedAt, e.ClosedAt, e.MergedAt, e.Additions, e.Deletions,
			e.URL, e.Mentions, e.Labels,
			e.State, e.Number, e.CreatedAt, e.ClosedAt, e.MergedAt, e.Additions, e.Deletions,
		)
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())
}

func TestQueryEntities(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	results, err := QueryEntities(db, "GOOGLE", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestGetEntity(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	result, err := GetEntity(db, "GOOGLE")
	require.NoError(t, err)
	assert.Equal(t, 2, result.DeveloperCount)
}

func TestGetEntityLike(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	results, err := GetEntityLike(db, "GOOG", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestGetEntityLike_EmptyQuery(t *testing.T) {
	db := setupTestDB(t)
	_, err := GetEntityLike(db, "", 10)
	assert.Error(t, err)
}

func TestGetEntityLike_NilDB(t *testing.T) {
	_, err := GetEntityLike(nil, "test", 10)
	assert.Error(t, err)
}

func TestQueryEntities_NilDB(t *testing.T) {
	_, err := QueryEntities(nil, "test", 10)
	assert.Error(t, err)
}

func TestGetEntity_NilDB(t *testing.T) {
	_, err := GetEntity(nil, "test")
	assert.Error(t, err)
}

func TestCleanEntities(t *testing.T) {
	db := setupTestDB(t)
	devs := []*Developer{
		{Username: "dev1", FullName: "Dev One", Entity: "Google LLC"},
		{Username: "dev2", FullName: "Dev Two", Entity: "Microsoft Corp"},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	require.NoError(t, CleanEntities(db))

	dev, err := GetDeveloper(db, "dev1")
	require.NoError(t, err)
	assert.Equal(t, "GOOGLE", dev.Entity)
}

func TestCleanEntities_NilDB(t *testing.T) {
	err := CleanEntities(nil)
	assert.Error(t, err)
}

func TestGetEntity_NotFound(t *testing.T) {
	db := setupTestDB(t)
	result, err := GetEntity(db, "NONEXISTENT")
	require.NoError(t, err)
	assert.Equal(t, 0, result.DeveloperCount)
}

func TestQueryEntities_NoMatch(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	results, err := QueryEntities(db, "NONEXISTENT", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}
