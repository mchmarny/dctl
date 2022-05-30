package data

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	queryEntitySQL = `SELECT 
			entity,  
			COUNT(DISTINCT username) as developers  
		FROM developer
		WHERE entity LIKE ?  
		GROUP BY 
			entity
		ORDER BY 2 DESC
		LIMIT ?
	`

	selectEntityDevelopersSQL = `SELECT 
			username,
			COALESCE(entity, '') AS entity,
			update_date 
		FROM developer 
		WHERE entity = ? 
		ORDER BY 1
	`

	selectEntityLike = `SELECT d.entity, COUNT(DISTINCT e.id) as event_count  
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
		return nil, errors.Wrap(err, "failed to prepare entity like statement")
	}

	query = fmt.Sprintf("%%%s%%", query)
	rows, err := stmt.Query(query, limit)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute select statement")
	}
	defer rows.Close()

	list := make([]*ListItem, 0)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
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
		return nil, errors.Wrap(err, "failed to prepare developer entity affiliation statement")
	}

	rows, err := stmt.Query(val)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute select statement")
	}
	defer rows.Close()

	list, err := mapDeveloperListItem(rows)
	if err != nil {
		return nil, errors.Wrap(err, "failed to map developer list")
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
		return nil, errors.Wrap(err, "failed to prepare developer entity statement")
	}

	val = fmt.Sprintf("%%%s%%", val)
	rows, err := stmt.Query(val, limit)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute select statement")
	}
	defer rows.Close()

	list := make([]*CountedItem, 0)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
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
		return errors.Wrap(err, "failed to prepare developer query statement")
	}

	m := make(map[string]string)
	rows, err := stmt.Query()
	if err != nil && err != sql.ErrNoRows {
		return errors.Wrap(err, "failed to execute select statement")
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return errors.Wrap(err, "failed to scan row")
		}
		m[name] = cleanEntityName(name)
	}

	updateStmt, err := db.Prepare(updateEntityNamesSQL)
	if err != nil {
		return errors.Wrap(err, "failed to prepare entity update statement")
	}

	tx, err := db.Begin()
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}

	for old, new := range m {
		if _, err = tx.Stmt(updateStmt).Exec(new, old); err != nil {
			rollbackTransaction(tx)
			return errors.Wrapf(err, "error updating entity %s to %s", old, new)
		}
	}

	if err = tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
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

	log.Debugf("cleaned entity name: %s -> %s", original, val)

	return val
}
