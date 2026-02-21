package data

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	err := Init(dbPath)
	require.NoError(t, err)
	db, err := GetDB(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInit_CreatesDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	err := Init(dbPath)
	require.NoError(t, err)
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestInit_EmptyPath(t *testing.T) {
	err := Init("")
	assert.Error(t, err)
}

func TestInit_RunsMigrations(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	require.NoError(t, Init(dbPath))
	db, err := GetDB(dbPath)
	require.NoError(t, err)
	defer db.Close()

	var version int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	assert.NoError(t, err)
	assert.Greater(t, version, 0)
}

func TestInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	require.NoError(t, Init(dbPath))
	assert.NoError(t, Init(dbPath))
}

func TestContains(t *testing.T) {
	assert.True(t, Contains([]string{"a", "b", "c"}, "b"))
	assert.False(t, Contains([]string{"a", "b"}, "d"))
	assert.False(t, Contains[string](nil, "a"))
}
