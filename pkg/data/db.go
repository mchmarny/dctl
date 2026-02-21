package data

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"regexp"

	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

const (
	DataFileName     string = "data.db"
	nonAlphaNumRegex string = "[^a-zA-Z0-9 ]+"
)

var (
	//go:embed sql/*
	f embed.FS

	errDBNotInitialized = errors.New("database not initialized")

	entityRegEx *regexp.Regexp
)

// Init initializes the database for a given name.
func Init(dbFilePath string) error {
	if dbFilePath == "" {
		return errors.New("dbFilePath not specified")
	}

	if _, err := os.Stat(dbFilePath); errors.Is(err, os.ErrNotExist) {
		db, err := GetDB(dbFilePath)
		if err != nil {
			return fmt.Errorf("error opening database: %s: %w", dbFilePath, err)
		}
		defer db.Close()

		log.Debug("creating db schema...")
		b, err := f.ReadFile("sql/ddl.sql")
		if err != nil {
			return fmt.Errorf("failed to read the schema creation file: %w", err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			return fmt.Errorf("failed to create database schema in: %s: %w", dbFilePath, err)
		}
		log.Debug("db schema created")
	}

	var err error
	entityRegEx, err = regexp.Compile(nonAlphaNumRegex)
	if err != nil {
		return fmt.Errorf("failed to compile entity regex: %w", err)
	}

	return nil
}

func GetDB(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %s: %w", path, err)
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
