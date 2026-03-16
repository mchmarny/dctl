package data

import (
	"database/sql"
	"fmt"
)

const (
	insertSubSQL = `INSERT INTO sub (type, old, new) VALUES (?, ?, ?) 
		ON CONFLICT(type, old) DO UPDATE SET new = ?
	`

	selectSubSQL = `SELECT type, old, new FROM sub`
)

func applyDeveloperSub(db *sql.DB, sub *Substitution) error {
	if db == nil {
		return ErrDBNotInitialized
	}

	if sub == nil {
		return nil
	}

	// CHeck if contains
	if !Contains(UpdatableProperties, sub.Prop) {
		return fmt.Errorf("invalid property: %s (permitted options: %v)", sub.Prop, UpdatableProperties)
	}

	stmt, err := db.Prepare(fmt.Sprintf(updateDeveloperPropertySQL, sub.Prop, sub.Prop))
	if err != nil {
		return fmt.Errorf("failed to prepare sql statement: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(sub.New, sub.Old)
	if err != nil {
		return fmt.Errorf("failed to execute developer property update statement: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	sub.Records = rows

	return nil
}

func SaveAndApplyDeveloperSub(db *sql.DB, prop, old, new string) (*Substitution, error) {
	if db == nil {
		return nil, ErrDBNotInitialized
	}

	s := &Substitution{
		Prop: prop,
		Old:  old,
		New:  new,
	}

	if err := applyDeveloperSub(db, s); err != nil {
		return nil, fmt.Errorf("failed to apply developer sub: %w", err)
	}

	subStmt, err := db.Prepare(insertSubSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare state insert statement: %w", err)
	}
	defer subStmt.Close()

	if _, err = subStmt.Exec(prop, old, new, new); err != nil {
		return nil, fmt.Errorf("failed to insert state: %w", err)
	}

	return s, nil
}

func ApplySubstitutions(db *sql.DB) ([]*Substitution, error) {
	if db == nil {
		return nil, ErrDBNotInitialized
	}

	stmt, err := db.Prepare(selectSubSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare sql statement: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, fmt.Errorf("failed to execute substitute select statement: %w", err)
	}
	defer rows.Close()

	list := make([]*Substitution, 0)
	for rows.Next() {
		s := &Substitution{}
		if err := rows.Scan(&s.Prop, &s.Old, &s.New); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		list = append(list, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	for _, s := range list {
		if err := applyDeveloperSub(db, s); err != nil {
			return nil, fmt.Errorf("failed to apply developer sub: %w", err)
		}
	}

	return list, nil
}
