package data

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	DataFileName     string = "data.db"
	nonAlphaNumRegex string = "[^a-zA-Z0-9 ]+"
)

var (
	//go:embed sql/migrations/*.sql
	migrationsFS embed.FS

	errDBNotInitialized = errors.New("database not initialized")

	entityRegEx *regexp.Regexp
)

// Init initializes the database for a given name.
func Init(dbFilePath string) error {
	if dbFilePath == "" {
		return errors.New("dbFilePath not specified")
	}

	db, err := GetDB(dbFilePath)
	if err != nil {
		return fmt.Errorf("opening database %s: %w", dbFilePath, err)
	}
	defer db.Close()

	if migErr := runMigrations(db); migErr != nil {
		return fmt.Errorf("running migrations: %w", migErr)
	}

	entityRegEx = regexp.MustCompile(nonAlphaNumRegex)

	return nil
}

func runMigrations(db *sql.DB) error {
	// Bootstrap schema_version table
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
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

		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", ver); err != nil {
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

func GetDB(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %s: %w", path, err)
	}

	// WAL mode on every connection (not just Init) so concurrent readers don't block.
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}

	// Wait up to 5s when the DB is locked instead of failing immediately.
	if _, err := conn.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("setting busy timeout: %w", err)
	}

	return conn, nil
}

// Contains checks for val in list
func Contains[T comparable](list []T, val T) bool {
	if list == nil {
		return false
	}
	for _, item := range list {
		if item == val {
			return true
		}
	}
	return false
}

type Query struct {
	On    int64  `json:"on,omitempty"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type CountedResult struct {
	Query   Query            `json:"query,omitempty"`
	Results int              `json:"results,omitempty"`
	Data    map[string]int64 `json:"data,omitempty"`
}
