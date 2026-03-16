package sqlite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func TestNew_CreatesDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := New(dbPath)
	require.NoError(t, err)
	defer store.Close()
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestNew_EmptyPath(t *testing.T) {
	_, err := New("")
	assert.Error(t, err)
}

func TestNew_RunsMigrations(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := New(dbPath)
	require.NoError(t, err)
	defer store.Close()

	var version int
	err = store.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	assert.NoError(t, err)
	assert.Greater(t, version, 0)
}

func TestNew_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store1, err := New(dbPath)
	require.NoError(t, err)
	store1.Close()
	store2, err := New(dbPath)
	assert.NoError(t, err)
	store2.Close()
}
