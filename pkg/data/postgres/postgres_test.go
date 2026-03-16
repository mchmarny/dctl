package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// sharedDSN holds the connection string for the single shared postgres container.
var sharedDSN string

// schemaSeq generates unique schema names across tests.
var schemaSeq atomic.Uint64

func TestMain(m *testing.M) {
	// Cannot call testing.Short() before flag.Parse(); check the flag directly.
	for _, arg := range os.Args[1:] {
		if arg == "-test.short" || arg == "-test.short=true" {
			os.Exit(m.Run())
		}
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
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
		os.Exit(1)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		container.Terminate(ctx)
		fmt.Fprintf(os.Stderr, "failed to get connection string: %v\n", err)
		os.Exit(1)
	}
	sharedDSN = dsn

	code := m.Run()

	container.Terminate(ctx)
	os.Exit(code)
}

func setupTestDB(t *testing.T) *Store {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping postgres integration test in short mode")
	}

	schema := fmt.Sprintf("test_%d", schemaSeq.Add(1))

	// Connect to the shared container and create an isolated schema.
	db, err := sql.Open("postgres", sharedDSN)
	require.NoError(t, err)

	_, err = db.Exec(fmt.Sprintf("CREATE SCHEMA %s", schema))
	require.NoError(t, err)

	_, err = db.Exec(fmt.Sprintf("SET search_path TO %s", schema))
	require.NoError(t, err)

	require.NoError(t, db.Close())

	// Build a DSN that sets search_path to the isolated schema.
	schemaDSN := sharedDSN
	if strings.Contains(schemaDSN, "?") {
		schemaDSN += "&search_path=" + schema
	} else {
		schemaDSN += "?search_path=" + schema
	}

	store, err := New(schemaDSN)
	require.NoError(t, err)

	t.Cleanup(func() {
		store.Close()
		// Drop the schema to free resources.
		cleanDB, err := sql.Open("postgres", sharedDSN)
		if err == nil {
			cleanDB.Exec(fmt.Sprintf("DROP SCHEMA %s CASCADE", schema))
			cleanDB.Close()
		}
	})

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
