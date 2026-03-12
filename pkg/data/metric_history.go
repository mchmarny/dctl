package data

import (
	"database/sql"
	"errors"
	"fmt"
)

const (
	upsertRepoMetricHistorySQL = `INSERT INTO repo_metric_history (org, repo, date, stars, forks)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(org, repo, date) DO UPDATE SET
			stars = ?, forks = ?
	`

	selectRepoMetricHistorySQL = `SELECT org, repo, date, stars, forks
		FROM repo_metric_history
		WHERE org = COALESCE(?, org)
		  AND repo = COALESCE(?, repo)
		ORDER BY org, repo, date
	`
)

type RepoMetricHistory struct {
	Org   string `json:"org"`
	Repo  string `json:"repo"`
	Date  string `json:"date"`
	Stars int    `json:"stars"`
	Forks int    `json:"forks"`
}

func GetRepoMetricHistory(db *sql.DB, org, repo *string) ([]*RepoMetricHistory, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	rows, err := db.Query(selectRepoMetricHistorySQL, org, repo)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query repo metric history: %w", err)
	}
	defer rows.Close()

	list := make([]*RepoMetricHistory, 0)
	for rows.Next() {
		m := &RepoMetricHistory{}
		if err := rows.Scan(&m.Org, &m.Repo, &m.Date, &m.Stars, &m.Forks); err != nil {
			return nil, fmt.Errorf("failed to scan repo metric history row: %w", err)
		}
		list = append(list, m)
	}

	return list, nil
}
