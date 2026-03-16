package postgres

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	_ "github.com/lib/pq"

	"github.com/mchmarny/devpulse/pkg/data"
)

// Compile-time check that Store implements data.Store.
var _ data.Store = (*Store)(nil)

//go:embed sql/migrations/*.sql
var migrationsFS embed.FS

// Store implements data.Store for PostgreSQL.
type Store struct {
	db *sql.DB
}

// New creates a new PostgreSQL Store, running migrations automatically.
func New(dsn string) (*Store, error) {
	if dsn == "" {
		return nil, fmt.Errorf("dsn not specified")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection pool.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB returns the underlying *sql.DB for cases that need direct access.
func (s *Store) DB() *sql.DB {
	return s.db
}

func runMigrations(db *sql.DB) error {
	// Bootstrap schema_version table
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT NOW()
	)`); err != nil {
		return fmt.Errorf("creating schema_version table: %w", err)
	}

	var currentVersion int
	if err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&currentVersion); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}

	entries, err := migrationsFS.ReadDir("sql/migrations")
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}

		var ver int
		if _, err := fmt.Sscanf(parts[0], "%d", &ver); err != nil {
			continue
		}

		if ver <= currentVersion {
			continue
		}

		content, err := migrationsFS.ReadFile("sql/migrations/" + name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}

		slog.Debug("applying migration", "version", ver, "file", name)

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning migration tx %d: %w", ver, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("executing migration %s: %w", name, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES ($1)", ver); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", ver, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %d: %w", ver, err)
		}

		slog.Info("applied migration", "version", ver, "file", name)
	}

	return nil
}
