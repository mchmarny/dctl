package data

import (
	"database/sql"

	"github.com/pkg/errors"
)

var (
	stateQueries = map[string]string{
		"developer":  "SELECT COUNT(*) FROM developer",
		"identity":   "SELECT COUNT(*) FROM identity",
		"entity":     "SELECT COUNT(*) FROM entity",
		"event":      "SELECT COUNT(*) FROM event",
		"event_type": "SELECT COUNT(DISTINCT event_type) FROM event",
	}

	insertState = `INSERT INTO state (query, org, repo, page) VALUES (?, ?, ?, ?)
		ON CONFLICT(query, org, repo) DO UPDATE SET page = ?
	`

	selectState = `SELECT page FROM state WHERE query = ? AND org = ? AND repo = ?`
)

func GetState(db *sql.DB, query, org, repo string) (int, error) {
	if db == nil {
		return 0, errDBNotInitialized
	}

	stateStmt, err := db.Prepare(selectState)
	if err != nil {
		return 0, errors.Wrap(err, "failed to prepare state select statement")
	}

	row := stateStmt.QueryRow(query, org, repo)

	var count sql.NullInt16
	err = row.Scan(&count)
	if err != nil {
		if err == sql.ErrNoRows {
			return 1, nil
		}
		return 0, errors.Wrap(err, "failed to scan row")
	}

	return int(count.Int16), nil
}

func SaveState(db *sql.DB, query, org, repo string, page int) error {
	if db == nil {
		return errDBNotInitialized
	}

	if query == "" || org == "" || repo == "" {
		return errors.Errorf("query: %s, org: %s, repo: %s are all required", query, org, repo)
	}

	stateStmt, err := db.Prepare(insertState)
	if err != nil {
		return errors.Wrapf(err, "failed to prepare state insert statement")
	}

	if _, err = stateStmt.Exec(query, org, repo, page, page); err != nil {
		return errors.Wrap(err, "failed to insert state")
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
			return nil, errors.Wrapf(err, "error preparing %s statement", k)
		}

		count, err := getCount(db, stmt)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting %s count", k)
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
		return 0, errors.Wrap(err, "failed to scan row")
	}

	return count, nil
}
