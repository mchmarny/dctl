package data

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	stateQueries = map[string]string{
		"developer": "SELECT COUNT(*) FROM developer",
		"identity":  "SELECT COUNT(*) FROM identity",
		"entity":    "SELECT COUNT(*) FROM entity",
		"event":     "SELECT COUNT(*) FROM event",
		"type":      "SELECT COUNT(DISTINCT type) FROM event",
	}

	insertState = `INSERT INTO state (query, org, repo, page, since) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(query, org, repo) DO UPDATE SET page = ?, since = ?
	`

	selectState = `SELECT since, page FROM state WHERE query = ? AND org = ? AND repo = ?`
)

type State struct {
	Since time.Time `json:"since"`
	Page  int       `json:"page"`
}

func GetState(db *sql.DB, query, org, repo string, min time.Time) (*State, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	stateStmt, err := db.Prepare(selectState)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare state select statement: %w", err)
	}

	row := stateStmt.QueryRow(query, org, repo)

	s := &State{
		Since: min,
		Page:  1,
	}
	var since int64
	err = row.Scan(&since, &s.Page)
	if err != nil {
		if err == sql.ErrNoRows {
			return s, nil
		}
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	s.Since = time.Unix(since, 0).UTC()

	return s, nil
}

func SaveState(db *sql.DB, query, org, repo string, state *State) error {
	if db == nil {
		return errDBNotInitialized
	}

	if state == nil {
		return errors.New("state is nil")
	}

	if query == "" || org == "" || repo == "" {
		return fmt.Errorf("query: %s, org: %s, repo: %s are all required", query, org, repo)
	}

	stateStmt, err := db.Prepare(insertState)
	if err != nil {
		return fmt.Errorf("failed to prepare state insert statement: %w", err)
	}

	since := state.Since.Unix()
	if _, err = stateStmt.Exec(query, org, repo, state.Page, state.Page, since, since); err != nil {
		return fmt.Errorf("failed to insert state: %w", err)
	}

	return nil
}

// GetDataState returns the current state of the database.
func GetDataState(db *sql.DB) (map[string]int64, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	state := make(map[string]int64)
	for k, v := range stateQueries {
		stmt, err := db.Prepare(v)
		if err != nil {
			return nil, fmt.Errorf("error preparing %s statement: %w", k, err)
		}

		count, err := getCount(db, stmt)
		if err != nil {
			return nil, fmt.Errorf("error getting %s count: %w", k, err)
		}
		state[k] = count
	}

	return state, nil
}

func getCount(db *sql.DB, stmt *sql.Stmt) (int64, error) {
	if db == nil {
		return 0, errDBNotInitialized
	}

	row := stmt.QueryRow()

	var count int64
	err := row.Scan(&count)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to scan row: %w", err)
	}

	return count, nil
}
