package postgres

import (
	"regexp"
	"testing"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedTestData(t *testing.T, store *Store) {
	t.Helper()
	devs := []*data.Developer{
		{Username: "dev1", FullName: "Dev One", Email: "dev1@google.com", Entity: "GOOGLE"},
		{Username: "dev2", FullName: "Dev Two", Email: "dev2@google.com", Entity: "GOOGLE"},
		{Username: "dev3", FullName: "Dev Three", Email: "dev3@msft.com", Entity: "MICROSOFT"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	tx, err := store.db.Begin()
	require.NoError(t, err)
	stmt, err := store.db.Prepare(insertEventSQL)
	require.NoError(t, err)

	events := []data.Event{
		{Org: "testorg", Repo: "testrepo", Username: "dev1", Type: data.EventTypePR, Date: "2026-01-15", URL: "https://github.com/pr/1", Mentions: "", Labels: ""},
		{Org: "testorg", Repo: "testrepo", Username: "dev2", Type: data.EventTypeIssue, Date: "2026-01-16", URL: "https://github.com/issue/1", Mentions: "dev1", Labels: "bug"},
		{Org: "testorg", Repo: "testrepo", Username: "dev3", Type: data.EventTypePR, Date: "2026-01-17", URL: "https://github.com/pr/2", Mentions: "", Labels: ""},
	}

	for _, e := range events {
		_, err = tx.Stmt(stmt).Exec(
			e.Org, e.Repo, e.Username, e.Type, e.Date,
			e.URL, e.Mentions, e.Labels,
			e.State, e.Number, e.CreatedAt, e.ClosedAt, e.MergedAt, e.Additions, e.Deletions,
			e.ChangedFiles, e.Commits, e.Title,
			e.URL, e.Mentions, e.Labels,
			e.State, e.Number, e.CreatedAt, e.ClosedAt, e.MergedAt, e.Additions, e.Deletions,
			e.ChangedFiles, e.Commits, e.Title,
		)
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())
}

func TestQueryEntities(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	results, err := store.QueryEntities("GOOGLE", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestGetEntity(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	result, err := store.GetEntity("GOOGLE")
	require.NoError(t, err)
	assert.Equal(t, 2, result.DeveloperCount)
}

func TestGetEntityLike(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	results, err := store.GetEntityLike("GOOG", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestGetEntityLike_EmptyQuery(t *testing.T) {
	store := setupTestDB(t)
	_, err := store.GetEntityLike("", 10)
	assert.Error(t, err)
}

func TestGetEntityLike_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetEntityLike("test", 10)
	assert.Error(t, err)
}

func TestQueryEntities_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.QueryEntities("test", 10)
	assert.Error(t, err)
}

func TestGetEntity_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetEntity("test")
	assert.Error(t, err)
}

func TestCleanEntities(t *testing.T) {
	store := setupTestDB(t)
	devs := []*data.Developer{
		{Username: "dev1", FullName: "Dev One", Entity: "Google LLC"},
		{Username: "dev2", FullName: "Dev Two", Entity: "Microsoft Corp"},
	}
	require.NoError(t, store.SaveDevelopers(devs))

	require.NoError(t, store.CleanEntities())

	dev, err := store.GetDeveloper("dev1")
	require.NoError(t, err)
	assert.Equal(t, "GOOGLE", dev.Entity)
}

func TestCleanEntities_NilDB(t *testing.T) {
	s := &Store{db: nil}
	err := s.CleanEntities()
	assert.Error(t, err)
}

func TestGetEntity_NotFound(t *testing.T) {
	store := setupTestDB(t)
	result, err := store.GetEntity("NONEXISTENT")
	require.NoError(t, err)
	assert.Equal(t, 0, result.DeveloperCount)
}

func TestQueryEntities_NoMatch(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	results, err := store.QueryEntities("NONEXISTENT", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

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
