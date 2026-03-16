package sqlite

import (
	"fmt"

	"github.com/mchmarny/devpulse/pkg/data"
)

const (
	insertSubSQL = `INSERT INTO sub (type, old, new) VALUES (?, ?, ?)
		ON CONFLICT(type, old) DO UPDATE SET new = ?
	`

	selectSubSQL = `SELECT type, old, new FROM sub`
)

func (s *Store) applyDeveloperSub(sub *data.Substitution) error {
	if s.db == nil {
		return data.ErrDBNotInitialized
	}

	if sub == nil {
		return nil
	}

	if !data.Contains(data.UpdatableProperties, sub.Prop) {
		return fmt.Errorf("invalid property: %s (permitted options: %v)", sub.Prop, data.UpdatableProperties)
	}

	stmt, err := s.db.Prepare(fmt.Sprintf(updateDeveloperPropertySQL, sub.Prop, sub.Prop))
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

func (s *Store) SaveAndApplyDeveloperSub(prop, old, new string) (*data.Substitution, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	sub := &data.Substitution{
		Prop: prop,
		Old:  old,
		New:  new,
	}

	if err := s.applyDeveloperSub(sub); err != nil {
		return nil, fmt.Errorf("failed to apply developer sub: %w", err)
	}

	subStmt, err := s.db.Prepare(insertSubSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare state insert statement: %w", err)
	}
	defer subStmt.Close()

	if _, err = subStmt.Exec(prop, old, new, new); err != nil {
		return nil, fmt.Errorf("failed to insert state: %w", err)
	}

	return sub, nil
}

func (s *Store) ApplySubstitutions() ([]*data.Substitution, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	stmt, err := s.db.Prepare(selectSubSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare sql statement: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, fmt.Errorf("failed to execute substitute select statement: %w", err)
	}
	defer rows.Close()

	list := make([]*data.Substitution, 0)
	for rows.Next() {
		sub := &data.Substitution{}
		if err := rows.Scan(&sub.Prop, &sub.Old, &sub.New); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		list = append(list, sub)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	for _, sub := range list {
		if err := s.applyDeveloperSub(sub); err != nil {
			return nil, fmt.Errorf("failed to apply developer sub: %w", err)
		}
	}

	return list, nil
}
