package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
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
			entity = ?
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

type Developer struct {
	Username      string `json:"username,omitempty" yaml:"username,omitempty"`
	FullName      string `json:"full_name,omitempty" yaml:"fullName,omitempty"`
	Email         string `json:"email,omitempty" yaml:"email,omitempty"`
	AvatarURL     string `json:"avatar,omitempty" yaml:"avatar,omitempty"`
	ProfileURL    string `json:"url,omitempty" yaml:"url,omitempty"`
	Entity        string `json:"entity,omitempty" yaml:"entity,omitempty"`
	Organizations []*Org `json:"organizations,omitempty" yaml:"organizations,omitempty"`
}

type DeveloperListItem struct {
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Entity   string `json:"entity,omitempty" yaml:"entity,omitempty"`
}

func GetDeveloperUsernames(db *sql.DB) ([]string, error) {
	return getDBSlice(db, selectDeveloperUsernameSQL)
}

func GetNoFullnameDeveloperUsernames(db *sql.DB) ([]string, error) {
	return getDBSlice(db, selectNoFullNameDeveloperUsernameSQL)
}

func getDBSlice(db *sql.DB, sqlQuery string) ([]string, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	stmt, err := db.Prepare(sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare sql statement: %w", err)
	}

	list := make([]string, 0)

	rows, err := stmt.Query()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

	return list, nil
}

func SaveDevelopers(db *sql.DB, devs []*Developer) error {
	if db == nil {
		return errDBNotInitialized
	}

	if len(devs) == 0 {
		return nil
	}

	userStmt, err := db.Prepare(insertDeveloperSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare developer insert statement: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	for i, u := range devs {
		if _, err = tx.Stmt(userStmt).Exec(u.Username,
			u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity,
			u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity); err != nil {
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

func UpdateDeveloper(ctx context.Context, db *sql.DB, client *http.Client, username string, cDev *CNCFDeveloper) error {
	if db == nil {
		return errDBNotInitialized
	}

	dbDev, err := GetDeveloper(db, username)
	if err != nil {
		return fmt.Errorf("failed to get developer %s: %w", username, err)
	}
	if dbDev == nil {
		return nil
	}

	// make sure the DB dev == CNCF dev
	if dbDev.Username != cDev.Username {
		return fmt.Errorf("username mismatch (CNCF): %s != %s", dbDev.Username, cDev.Username)
	}

	ghDev, err := GetGitHubDeveloper(ctx, client, username)
	if err != nil {
		return fmt.Errorf("failed to get github developer %s: %w", username, err)
	}

	if dbDev.Username != ghDev.Username {
		return fmt.Errorf("username mismatch (GitHub): %s != %s", dbDev.Username, ghDev.Username)
	}

	printDevDeltas(dbDev, ghDev, cDev)

	// always update from GH first (in case they changed their name)
	if ghDev.Email != "" {
		dbDev.Email = ghDev.Email
	}

	if ghDev.FullName != "" {
		dbDev.FullName = ghDev.FullName
	}

	if ghDev.AvatarURL != "" {
		dbDev.AvatarURL = ghDev.AvatarURL
	}

	dbDev.Entity = cleanEntityName(ghDev.Entity)

	// The CNCF entity affiliation is the best source.
	// Update regardless if the GitHub profile has anything or not.
	ca := cDev.GetLatestAffiliation()
	if ca != "" {
		dbDev.Entity = ca
	}

	// GitHub validates emails so only update if the developer doesn't already have
	if len(dbDev.Email) == 0 {
		dbDev.Email = cleanEntityName(cDev.GetBestIdentity())
	}

	// update the DB
	if err := SaveDevelopers(db, []*Developer{dbDev}); err != nil {
		return fmt.Errorf("failed to save developer %s: %w", username, err)
	}

	return nil
}

func printDevDeltas(dbDev *Developer, ghDev *Developer, cncfDev *CNCFDeveloper) {
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

func GetDeveloper(db *sql.DB, username string) (*Developer, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	stmt, err := db.Prepare(selectDeveloperSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare developer select statement: %w", err)
	}

	row := stmt.QueryRow(username)

	u := &Developer{}
	if err = row.Scan(&u.Username, &u.FullName, &u.Email, &u.AvatarURL, &u.ProfileURL, &u.Entity); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	return u, nil
}

func mapDeveloperListItem(rows *sql.Rows) ([]*DeveloperListItem, error) {
	list := make([]*DeveloperListItem, 0)
	for rows.Next() {
		u := &DeveloperListItem{}
		if err := rows.Scan(&u.Username, &u.Entity); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return list, nil
			}
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		list = append(list, u)
	}
	return list, nil
}

// SearchDevelopers returns a list of developers matching the given query.
func SearchDevelopers(db *sql.DB, val string, limit int) ([]*DeveloperListItem, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	stmt, err := db.Prepare(queryDeveloperSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare developer query statement: %w", err)
	}

	val = fmt.Sprintf("%%%s%%", val)
	rows, err := stmt.Query(val, val, val, limit)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	return mapDeveloperListItem(rows)
}

func UpdateDeveloperNames(db *sql.DB, devs map[string]string) error {
	if db == nil {
		return errDBNotInitialized
	}

	updateStmt, err := db.Prepare(updateDeveloperNamesSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare entity update statement: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	for username, name := range devs {
		if _, err = tx.Stmt(updateStmt).Exec(name, username); err != nil {
			rollbackTransaction(tx)
			return fmt.Errorf("error updating full name for %s to %s: %w", username, name, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
