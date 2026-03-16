package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mchmarny/devpulse/pkg/data"
)

var stateQueries = map[string]string{
	"developer": "SELECT COUNT(*) FROM developer",
	"identity":  "SELECT COUNT(*) FROM identity",
	"entity":    "SELECT COUNT(*) FROM entity",
	"event":     "SELECT COUNT(*) FROM event",
	"type":      "SELECT COUNT(DISTINCT type) FROM event",
}

const (
	insertStateSQL = `INSERT INTO state (query, org, repo, page, since) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(query, org, repo) DO UPDATE SET page = ?, since = ?
	`

	selectStateSQL = `SELECT since, page FROM state WHERE query = ? AND org = ? AND repo = ?`
)

func (s *Store) GetState(query, org, repo string, min time.Time) (*data.State, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	stateStmt, err := s.db.Prepare(selectStateSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare state select statement: %w", err)
	}
	defer stateStmt.Close()

	row := stateStmt.QueryRow(query, org, repo)

	st := &data.State{
		Since: min,
		Page:  1,
	}
	var since int64
	err = row.Scan(&since, &st.Page)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return st, nil
		}
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	st.Since = time.Unix(since, 0).UTC()

	return st, nil
}

func (s *Store) SaveState(query, org, repo string, state *data.State) error {
	if s.db == nil {
		return data.ErrDBNotInitialized
	}

	if state == nil {
		return errors.New("state is nil")
	}

	if query == "" || org == "" || repo == "" {
		return fmt.Errorf("query: %s, org: %s, repo: %s are all required", query, org, repo)
	}

	stateStmt, err := s.db.Prepare(insertStateSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare state insert statement: %w", err)
	}
	defer stateStmt.Close()

	since := state.Since.Unix()
	if _, err = stateStmt.Exec(query, org, repo, state.Page, since, state.Page, since); err != nil {
		return fmt.Errorf("failed to insert state: %w", err)
	}

	return nil
}

func (s *Store) ClearState(org, repo string) error {
	if s.db == nil {
		return data.ErrDBNotInitialized
	}

	q := "DELETE FROM state WHERE org = ? AND repo = ?"
	if _, err := s.db.Exec(q, org, repo); err != nil {
		return fmt.Errorf("failed to clear state for %s/%s: %w", org, repo, err)
	}

	return nil
}

func (s *Store) GetDataState() (map[string]int64, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	state := make(map[string]int64)
	for k, v := range stateQueries {
		stmt, err := s.db.Prepare(v)
		if err != nil {
			return nil, fmt.Errorf("error preparing %s statement: %w", k, err)
		}
		defer stmt.Close()

		count, err := getCount(stmt)
		if err != nil {
			return nil, fmt.Errorf("error getting %s count: %w", k, err)
		}
		state[k] = count
	}

	return state, nil
}

func getCount(stmt *sql.Stmt) (int64, error) {
	row := stmt.QueryRow()

	var count int64
	err := row.Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to scan row: %w", err)
	}

	return count, nil
}
