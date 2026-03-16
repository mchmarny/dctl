package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/mchmarny/devpulse/pkg/data"
)

const (
	insertDeveloperSQL = `INSERT INTO developer (
			username,
			full_name,
			email,
			avatar,
			url,
			entity
		)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(username) DO UPDATE SET
			full_name = ?,
			email = ?,
			avatar = ?,
			url = ?,
			entity = CASE WHEN ? = '' THEN COALESCE(developer.entity, '') ELSE ? END
	`

	selectDeveloperSQL = `SELECT
			username,
			full_name,
			email,
			avatar,
			url,
			entity
		FROM developer
		WHERE username = ?
	`

	selectDeveloperUsernameSQL = `SELECT DISTINCT username FROM developer`

	selectNoFullNameDeveloperUsernameSQL = `SELECT DISTINCT username
		FROM developer
		WHERE full_name IS NULL
		OR full_name = ''
	`

	queryDeveloperSQL = `SELECT
			username,
			COALESCE(entity, '') AS entity
		FROM developer
		WHERE username LIKE ?
		OR email LIKE ?
		OR entity LIKE ?
		LIMIT ?
	`

	updateDeveloperNamesSQL = `UPDATE developer SET full_name = ? WHERE username = ?`

	updateDeveloperPropertySQL = `UPDATE developer SET %s = ? WHERE %s = ?`
)

func (s *Store) GetDeveloperUsernames() ([]string, error) {
	return s.getDBSlice(selectDeveloperUsernameSQL)
}

func (s *Store) GetNoFullnameDeveloperUsernames() ([]string, error) {
	return s.getDBSlice(selectNoFullNameDeveloperUsernameSQL)
}

func (s *Store) getDBSlice(sqlQuery string) ([]string, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	stmt, err := s.db.Prepare(sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare sql statement: %w", err)
	}
	defer stmt.Close()

	list := make([]string, 0)

	rows, err := stmt.Query()
	if err != nil {
		return nil, fmt.Errorf("failed to execute series select statement: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		list = append(list, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}

func (s *Store) SaveDevelopers(devs []*data.Developer) error {
	if s.db == nil {
		return data.ErrDBNotInitialized
	}

	if len(devs) == 0 {
		return nil
	}

	userStmt, err := s.db.Prepare(insertDeveloperSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare developer insert statement: %w", err)
	}
	defer userStmt.Close()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	txStmt := tx.Stmt(userStmt)
	for i, u := range devs {
		if _, err = txStmt.Exec(u.Username,
			u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity,
			u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity, u.Entity); err != nil {
			slog.Error("failed to insert developer",
				"index", i,
				"error", err,
				"user", u.Username,
				"name", u.FullName,
				"email", u.Email,
				"avatar", u.AvatarURL,
				"profile", u.ProfileURL,
				"entity", u.Entity,
			)
			rollbackTransaction(tx)
			return fmt.Errorf("error inserting developer[%d]: %s: %w", i, u.Username, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Store) MergeDeveloper(ctx context.Context, client *http.Client, username string, cDev *data.CNCFDeveloper) (*data.Developer, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	dbDev, err := s.GetDeveloper(username)
	if err != nil {
		return nil, fmt.Errorf("failed to get developer %s: %w", username, err)
	}
	if dbDev == nil {
		return nil, nil
	}

	if dbDev.Username != cDev.Username {
		return nil, fmt.Errorf("username mismatch (CNCF): %s != %s", dbDev.Username, cDev.Username)
	}

	ghDev, err := GetGitHubDeveloper(ctx, client, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get github developer %s: %w", username, err)
	}

	if dbDev.Username != ghDev.Username {
		return nil, fmt.Errorf("username mismatch (GitHub): %s != %s", dbDev.Username, ghDev.Username)
	}

	printDevDeltas(dbDev, ghDev, cDev)

	if ghDev.Email != "" {
		dbDev.Email = ghDev.Email
	}

	if ghDev.FullName != "" {
		dbDev.FullName = ghDev.FullName
	}

	if ghDev.AvatarURL != "" {
		dbDev.AvatarURL = ghDev.AvatarURL
	}

	ghEntity := cleanEntityName(ghDev.Entity)
	if ghEntity != "" {
		dbDev.Entity = ghEntity
	} else if ca := cDev.GetLatestAffiliation(); ca != "" {
		dbDev.Entity = ca
	}

	if len(dbDev.Email) == 0 {
		dbDev.Email = cleanEntityName(cDev.GetBestIdentity())
	}

	return dbDev, nil
}

func printDevDeltas(dbDev *data.Developer, ghDev *data.Developer, cncfDev *data.CNCFDeveloper) {
	slog.Debug("developer deltas",
		"username", dbDev.Username,
		"entity_db", dbDev.Entity,
		"entity_gh", ghDev.Entity,
		"entity_cncf", cncfDev.GetLatestAffiliation(),
		"email_db", dbDev.Email,
		"email_gh", ghDev.Email,
		"email_cncf", cncfDev.GetBestIdentity(),
	)
}

func (s *Store) GetDeveloper(username string) (*data.Developer, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	stmt, err := s.db.Prepare(selectDeveloperSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare developer select statement: %w", err)
	}
	defer stmt.Close()

	row := stmt.QueryRow(username)

	u := &data.Developer{}
	if err = row.Scan(&u.Username, &u.FullName, &u.Email, &u.AvatarURL, &u.ProfileURL, &u.Entity); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	return u, nil
}

func (s *Store) SearchDevelopers(val string, limit int) ([]*data.DeveloperListItem, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	stmt, err := s.db.Prepare(queryDeveloperSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare developer query statement: %w", err)
	}
	defer stmt.Close()

	val = fmt.Sprintf("%%%s%%", val)
	rows, err := stmt.Query(val, val, val, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	return mapDeveloperListItem(rows)
}

func (s *Store) UpdateDeveloperNames(devs map[string]string) error {
	if s.db == nil {
		return data.ErrDBNotInitialized
	}

	updateStmt, err := s.db.Prepare(updateDeveloperNamesSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare entity update statement: %w", err)
	}
	defer updateStmt.Close()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	txStmt2 := tx.Stmt(updateStmt)
	for username, name := range devs {
		if _, err = txStmt2.Exec(name, username); err != nil {
			rollbackTransaction(tx)
			return fmt.Errorf("error updating full name for %s to %s: %w", username, name, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
