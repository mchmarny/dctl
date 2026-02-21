package data

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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

	selectEntityLike = `SELECT d.entity, COUNT(*) as event_count  
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

type EntityResult struct {
	Entity         string               `json:"entity,omitempty"`
	DeveloperCount int                  `json:"developer_count,omitempty"`
	Developers     []*DeveloperListItem `json:"developers,omitempty"`
}

// GetEntityLike returns a list of repos that match the given pattern.
func GetEntityLike(db *sql.DB, query string, limit int) ([]*ListItem, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	if query == "" {
		return nil, errors.New("query is required")
	}

	stmt, err := db.Prepare(selectEntityLike)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare entity like statement: %w", err)
	}

	query = fmt.Sprintf("%%%s%%", query)
	rows, err := stmt.Query(query, limit)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	list := make([]*ListItem, 0)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		e := &ListItem{
			Value: name,
			Text:  fmt.Sprintf("%s (%d events)", name, count),
		}
		list = append(list, e)
	}

	return list, nil
}

func GetEntity(db *sql.DB, val string) (*EntityResult, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	stmt, err := db.Prepare(selectEntityDevelopersSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare developer entity affiliation statement: %w", err)
	}

	rows, err := stmt.Query(val)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	list, err := mapDeveloperListItem(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to map developer list: %w", err)
	}

	r := &EntityResult{
		Entity:         val,
		DeveloperCount: len(list),
		Developers:     list,
	}

	return r, nil
}

func QueryEntities(db *sql.DB, val string, limit int) ([]*CountedItem, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	stmt, err := db.Prepare(queryEntitySQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare developer entity statement: %w", err)
	}

	val = fmt.Sprintf("%%%s%%", val)
	rows, err := stmt.Query(val, limit)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	list := make([]*CountedItem, 0)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		list = append(list, &CountedItem{
			Name:  name,
			Count: count,
		})
	}

	return list, nil
}

func CleanEntities(db *sql.DB) error {
	if db == nil {
		return errDBNotInitialized
	}

	stmt, err := db.Prepare(selectEntityNamesSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare developer query statement: %w", err)
	}

	m := make(map[string]string)
	rows, err := stmt.Query()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

	updateStmt, err := db.Prepare(updateEntityNamesSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare entity update statement: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	for old, new := range m {
		if _, err = tx.Stmt(updateStmt).Exec(new, old); err != nil {
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
	// get everything trimmed and upper cased
	val = strings.ToUpper(strings.TrimSpace(val))

	// substitute any known aliases
	if name, ok := entitySubstitutions[val]; ok {
		val = name
	}

	// remove any non-alphanumeric characters
	val = entityRegEx.ReplaceAllString(val, "")

	// split remaining string into words
	parts := make([]string, 0)

	// remove any part that's in the entity noise map
	for _, part := range strings.Split(val, " ") {
		if len(strings.ToUpper(strings.TrimSpace(part))) == 0 {
			continue
		}
		if _, ok := entityNoise[part]; !ok {
			parts = append(parts, part)
		}
	}

	// put it all back together
	val = strings.Join(parts, " ")

	// substitute any known aliases again, in case we fixed something
	if name, ok := entitySubstitutions[val]; ok {
		val = name
	}

	if len(val) > 0 {
		slog.Debug("cleaned entity name", "original", original, "cleaned", val)
	}

	return val
}
