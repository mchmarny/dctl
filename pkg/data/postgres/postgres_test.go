package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) *Store {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping postgres integration test in short mode")
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("devpulse_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	store, err := New(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func TestNew_ConnectsAndMigrates(t *testing.T) {
	store := setupTestDB(t)

	var version int
	err := store.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	assert.NoError(t, err)
	assert.Greater(t, version, 0)
}

func TestNew_EmptyDSN(t *testing.T) {
	_, err := New("")
	assert.Error(t, err)
}

func TestNew_BadDSN(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping postgres integration test in short mode")
	}
	_, err := New("postgres://invalid:invalid@localhost:1/nonexistent?sslmode=disable&connect_timeout=1")
	assert.Error(t, err)
}
