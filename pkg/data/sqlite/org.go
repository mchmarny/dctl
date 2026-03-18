package sqlite

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mchmarny/devpulse/pkg/data"
)

const (
	selectOrgEntityPercentSQL = `SELECT
			entity,
			ROUND(100.0 * events / (SUM(events) OVER ())) AS percent
		FROM (
			SELECT
				d.entity,
				COUNT(*) as events
			FROM developer d
			JOIN event e ON d.username = e.username
			WHERE (d.entity <> '' OR d.entity is null)
			AND e.date >= ?
			AND d.entity = COALESCE(?, d.entity)
			AND e.org = COALESCE(?, e.org)
			AND e.repo = COALESCE(?, e.repo)
			AND d.entity NOT IN (%s)
			` + forkExcludeSQL + `
			GROUP BY d.entity
		) dt
		ORDER BY 2 DESC
	`

	selectDeveloperPercentSQL = `SELECT
			username,
			ROUND(100.0 * events / (SUM(events) OVER ())) AS percent
		FROM (
			SELECT
				d.username,
				COUNT(*) as events
			FROM developer d
			JOIN event e ON d.username = e.username
			WHERE e.date >= ?
			AND d.entity = COALESCE(?, d.entity)
			AND e.org = COALESCE(?, e.org)
			AND e.repo = COALESCE(?, e.repo)
			AND d.username NOT IN (%s)
			AND d.username NOT LIKE '%%[bot]'
			` + forkExcludeSQL + `
			GROUP BY d.username
		) dt
		ORDER BY 2 DESC
	`

	selectOrgLikeSQL = `SELECT org, COUNT(DISTINCT repo) as repo_count, COUNT(*) as event_count
		FROM event
		WHERE org like ?
		GROUP BY org
		ORDER BY org DESC
		LIMIT ?
	`

	selectAllOrgReposSQL = `SELECT DISTINCT org, repo FROM event ORDER BY 1, 2`

	selectDeveloperSearchSQL = `SELECT DISTINCT d.username
		FROM developer d
		JOIN event e ON d.username = e.username
		WHERE d.username LIKE ?
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND d.username NOT LIKE '%[bot]'
		  AND e.date >= ?
		  ` + forkExcludeSQL + `
		ORDER BY d.username
		LIMIT ?
	`
)

func (s *Store) GetAllOrgRepos() ([]*data.OrgRepoItem, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	stmt, err := s.db.Prepare(selectAllOrgReposSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare developer percentages statement: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	list := make([]*data.OrgRepoItem, 0)
	for rows.Next() {
		e := &data.OrgRepoItem{}
		if err := rows.Scan(&e.Org, &e.Repo); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		list = append(list, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}

func (s *Store) getPercentages(sqlStr string, entity, org, repo *string, ex []string, months int) ([]*data.CountedItem, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	params := make([]string, len(ex))
	qArgs := []interface{}{since, entity, org, repo}

	for i, v := range ex {
		params[i] = "?"
		qArgs = append(qArgs, v)
	}

	stmt, err := s.db.Prepare(fmt.Sprintf(sqlStr, strings.Join(params, ",")))
	if err != nil {
		return nil, fmt.Errorf("failed to prepare percentages statement: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query(qArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	list := make([]*data.CountedItem, 0)
	for rows.Next() {
		e := &data.CountedItem{}
		if err := rows.Scan(&e.Name, &e.Count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		list = append(list, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}

func (s *Store) GetDeveloperPercentages(entity, org, repo *string, ex []string, months int) ([]*data.CountedItem, error) {
	return s.getPercentages(selectDeveloperPercentSQL, entity, org, repo, ex, months)
}

func (s *Store) GetEntityPercentages(entity, org, repo *string, ex []string, months int) ([]*data.CountedItem, error) {
	return s.getPercentages(selectOrgEntityPercentSQL, entity, org, repo, ex, months)
}

func (s *Store) SearchDeveloperUsernames(query string, org, repo *string, months, limit int) ([]string, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	since := sinceDate(months)
	pattern := fmt.Sprintf("%%%s%%", query)

	rows, err := s.db.Query(selectDeveloperSearchSQL, pattern, org, repo, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search developers: %w", err)
	}
	defer rows.Close()

	list := make([]string, 0)
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, fmt.Errorf("failed to scan developer row: %w", err)
		}
		list = append(list, username)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}

func (s *Store) GetOrgLike(query string, limit int) ([]*data.ListItem, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	if query == "" {
		return nil, errors.New("query is required")
	}

	stmt, err := s.db.Prepare(selectOrgLikeSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare org like statement: %w", err)
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
		e := &data.ListItem{}
		var repoCount, eventCount int
		if err := rows.Scan(&e.Value, &repoCount, &eventCount); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		e.Text = fmt.Sprintf("%s (%d repos, %d events)", e.Value, repoCount, eventCount)
		list = append(list, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}
