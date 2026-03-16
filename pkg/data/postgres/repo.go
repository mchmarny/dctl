package postgres

import (
	"errors"
	"fmt"

	"github.com/mchmarny/devpulse/pkg/data"
)

const (
	selectRepoLikeSQL = `SELECT org, repo, COUNT(*) as event_count
		FROM event
		WHERE repo ILIKE $1
		GROUP BY org, repo
		ORDER BY org DESC, repo DESC
		LIMIT $2
	`
)

func (s *Store) GetRepoLike(query string, limit int) ([]*data.ListItem, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	if query == "" {
		return nil, errors.New("query is required")
	}

	stmt, err := s.db.Prepare(selectRepoLikeSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare repo like statement: %w", err)
	}
	defer stmt.Close()

	query = fmt.Sprintf("%%%s%%", query)
	rows, err := stmt.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	list := make([]*data.ListItem, 0)
	for rows.Next() {
		var org, repo string
		var count int
		if err := rows.Scan(&org, &repo, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		e := &data.ListItem{
			Value: fmt.Sprintf("%s/%s", org, repo),
			Text:  fmt.Sprintf("%s/%s (%d events)", org, repo, count),
		}
		list = append(list, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}
