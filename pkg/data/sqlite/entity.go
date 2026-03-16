package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mchmarny/devpulse/pkg/data"
)

const (
	queryEntitySQL = `SELECT
			entity,
			COUNT(*) as developers
		FROM developer
		WHERE entity LIKE ?
		GROUP BY
			entity
		ORDER BY 2 DESC
		LIMIT ?
	`

	selectEntityDevelopersSQL = `SELECT
			username,
			COALESCE(entity, '') AS entity
		FROM developer
		WHERE entity = ?
		ORDER BY 1
	`

	selectEntityLikeSQL = `SELECT d.entity, COUNT(*) as event_count
		FROM developer d
		JOIN event e ON d.username = e.username
		WHERE d.entity like ?
		GROUP BY d.entity
		ORDER BY d.entity DESC
		LIMIT ?
	`

	selectEntityNamesSQL = `SELECT DISTINCT entity FROM developer WHERE entity IS NOT NULL and entity != ''`

	updateEntityNamesSQL = `UPDATE developer SET entity = ? WHERE entity = ?`
)

func (s *Store) GetEntityLike(query string, limit int) ([]*data.ListItem, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	if query == "" {
		return nil, errors.New("query is required")
	}

	stmt, err := s.db.Prepare(selectEntityLikeSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare entity like statement: %w", err)
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
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		e := &data.ListItem{
			Value: name,
			Text:  fmt.Sprintf("%s (%d events)", name, count),
		}
		list = append(list, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}

func (s *Store) GetEntity(val string) (*data.EntityResult, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	stmt, err := s.db.Prepare(selectEntityDevelopersSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare developer entity affiliation statement: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query(val)
	if err != nil {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	list, err := mapDeveloperListItem(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to map developer list: %w", err)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	r := &data.EntityResult{
		Entity:         val,
		DeveloperCount: len(list),
		Developers:     list,
	}

	return r, nil
}

func (s *Store) QueryEntities(val string, limit int) ([]*data.CountedItem, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	stmt, err := s.db.Prepare(queryEntitySQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare developer entity statement: %w", err)
	}
	defer stmt.Close()

	val = fmt.Sprintf("%%%s%%", val)
	rows, err := stmt.Query(val, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	list := make([]*data.CountedItem, 0)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		list = append(list, &data.CountedItem{
			Name:  name,
			Count: count,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}

func (s *Store) CleanEntities() error {
	if s.db == nil {
		return data.ErrDBNotInitialized
	}

	stmt, err := s.db.Prepare(selectEntityNamesSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare developer query statement: %w", err)
	}
	defer stmt.Close()

	m := make(map[string]string)
	rows, err := stmt.Query()
	if err != nil {
		return fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		m[name] = cleanEntityName(name)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	updateStmt, err := s.db.Prepare(updateEntityNamesSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare entity update statement: %w", err)
	}
	defer updateStmt.Close()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	txStmt := tx.Stmt(updateStmt)
	for old, new := range m {
		if _, err = txStmt.Exec(new, old); err != nil {
			rollbackTransaction(tx)
			return fmt.Errorf("error updating entity %s to %s: %w", old, new, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func cleanEntityName(val string) string {
	original := val
	val = strings.ToUpper(strings.TrimSpace(val))

	if name, ok := entitySubstitutions[val]; ok {
		val = name
	}

	val = entityRegEx.ReplaceAllString(val, "")

	parts := make([]string, 0)
	for _, part := range strings.Split(val, " ") {
		if len(strings.ToUpper(strings.TrimSpace(part))) == 0 {
			continue
		}
		if _, ok := entityNoise[part]; !ok {
			parts = append(parts, part)
		}
	}

	val = strings.Join(parts, " ")

	if name, ok := entitySubstitutions[val]; ok {
		val = name
	}

	if len(val) > 0 {
		slog.Debug("cleaned entity name", "original", original, "cleaned", val)
	}

	return val
}

func mapDeveloperListItem(rows *sql.Rows) ([]*data.DeveloperListItem, error) {
	list := make([]*data.DeveloperListItem, 0)
	for rows.Next() {
		u := &data.DeveloperListItem{}
		if err := rows.Scan(&u.Username, &u.Entity); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		list = append(list, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}
