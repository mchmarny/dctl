package data

import (
	"database/sql"
	"embed"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

const (
	dataFile string = "data.db"
)

var (
	db *sql.DB

	//go:embed sql/*
	f embed.FS
)

// New initializes the database for a given name.
func Init(dir string) error {
	if dir == "" {
		return errors.New("directory not specified")
	}

	wasCreated := false
	dataPath := filepath.Join(dir, dataFile)
	log.Debug().Msgf("data path: %s", dataPath)

	if _, err := os.Stat(dataPath); errors.Is(err, os.ErrNotExist) {
		log.Debug().Msg("data file does not exist, creating...")
		wasCreated = true
	}

	var err error
	db, err = sql.Open("sqlite3", dataPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open database: %s", dataPath)
	}

	if wasCreated {
		log.Debug().Msg("creating schema...")

		b, err := f.ReadFile("sql/ddl.sql")
		if err != nil {
			return errors.Wrap(err, "failed to read the schema creation file")
		}
		if _, err := db.Exec(string(b)); err != nil {
			return errors.Wrapf(err, "failed to create database schema in: %s", dataPath)
		}
	}

	log.Debug().Msg("data initialized")
	return nil
}

// Close closes the database if one of previously created.
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
