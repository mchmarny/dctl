package data

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	insertDeveloperSQL = `INSERT INTO developer (
			username,
			update_date,
			id,
			full_name,
			email,
			avatar_url,
			profile_url,
			entity,
			location
		) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) 
		ON CONFLICT(username) DO UPDATE SET 
			update_date = ?,
			id = ?,
			full_name = ?,
			email = ?,
			avatar_url = ?,
			profile_url = ?,
			entity = ?,
			location = ?
	`

	selectDeveloperSQL = `SELECT
			username,
			update_date,
			id,
			full_name,
			email,
			avatar_url,
			profile_url,
			entity,
			location
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
			COALESCE(entity, '') AS entity,
			update_date
		FROM developer
		WHERE username LIKE ? 
		OR email LIKE ? 
		OR entity LIKE ? 
		LIMIT ?
	`

	updateDeveloperNamesSQL = `UPDATE developer SET full_name = ? WHERE username = ?`
)

var (
	entityNoise = map[string]bool{
		"B.V.":        true,
		"CDL":         true,
		"CO":          true,
		"COMPANY":     true,
		"CORP":        true,
		"CORPORATION": true,
		"GMBH":        true,
		"GROUP":       true,
		"INC":         true,
		"LLC":         true,
		"LTD":         true,
		"PVT":         true,
		"SE":          true,
	}

	entitySubstitutions = map[string]string{
		"INTERNATIONAL BUSINESS MACHINES CORPORATION": "IBM",
		"INTERNATIONAL BUSINESS MACHINES":             "IBM",
		"S&P GLOBAL INC":                              "S&P",
		"S&P GLOBAL":                                  "S&P",
		"VERVERICA ORIGINAL CREATORS OF APACHE FLINK": "VERVERICA",
	}
)

type Developer struct {
	Username      string `json:"username,omitempty"`
	Updated       string `json:"update_date,omitempty"`
	ID            int64  `json:"id,omitempty"`
	FullName      string `json:"full_name,omitempty"`
	Email         string `json:"email,omitempty"`
	AvatarURL     string `json:"avatar_url,omitempty"`
	ProfileURL    string `json:"profile_url,omitempty"`
	Entity        string `json:"current_entity,omitempty"`
	Location      string `json:"location,omitempty"`
	Organizations []*Org `json:"organizations,omitempty"`
}

type DeveloperListItem struct {
	Username    string `json:"username,omitempty"`
	Entity      string `json:"entity,omitempty"`
	UpdatedDate string `json:"update_date,omitempty"`
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
		return nil, errors.Wrap(err, "failed to prepare sql statement")
	}

	list := make([]string, 0)

	rows, err := stmt.Query()
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute series select statement")
	}
	defer rows.Close()

	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
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
		return errors.Wrap(err, "failed to prepare developer insert statement")
	}

	tx, err := db.Begin()
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}

	for i, u := range devs {
		if _, err = tx.Stmt(userStmt).Exec(u.Username,
			u.Updated, u.ID, u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity, u.Location,
			u.Updated, u.ID, u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity, u.Location); err != nil {
			log.WithFields(log.Fields{
				"user":    u.Username,
				"updated": u.Updated,
				"id":      u.ID,
				"name":    u.FullName,
				"email":   u.Email,
				"avatar":  u.AvatarURL,
				"profile": u.ProfileURL,
				"entity":  u.Entity,
				"loc":     u.Location,
			}).Errorf("failed to insert developer[%d]: %v", i, err)
			rollbackTransaction(tx)
			return errors.Wrapf(err, "error inserting developer[%d]: %s", i, u.Username)
		}
	}

	if err = tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}

func UpdateDeveloper(ctx context.Context, db *sql.DB, client *http.Client, username string, cDev *CNCFDeveloper) error {
	if db == nil {
		return errDBNotInitialized
	}

	dbDev, err := GetDeveloper(db, username)
	if err != nil {
		return errors.Wrapf(err, "failed to get developer %s", username)
	}
	if dbDev == nil {
		return nil
	}

	// make sure the DB dev == CNCF dev
	if dbDev.Username != cDev.Username {
		return errors.Errorf("username mismatch (CNCF): %s != %s", dbDev.Username, cDev.Username)
	}

	ghDev, err := GetGitHubDeveloper(ctx, client, username)
	if err != nil {
		return errors.Wrapf(err, "failed to get github developer %s", username)
	}

	if dbDev.Username != ghDev.Username {
		return errors.Errorf("username mismatch (GitHub): %s != %s", dbDev.Username, ghDev.Username)
	}

	printDevDeltas(dbDev, ghDev, cDev)

	// update the date
	dbDev.Updated = time.Now().UTC().Format("2006-01-02")

	// always update from GH first (in case they changed their name)
	if ghDev.Email != "" {
		dbDev.Email = ghDev.Email
	}

	if ghDev.FullName != "" {
		dbDev.FullName = ghDev.FullName
	}

	if ghDev.AvatarURL != "" {
		dbDev.Location = ghDev.Location
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
		return errors.Wrapf(err, "failed to save developer %s", username)
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

func printDevDeltas(dbDev *Developer, ghDev *Developer, cncfDev *CNCFDeveloper) {
	log.Debugf("%s [entity db:%s, gh:%s, cncf:%s], email [db:%s, gh:%s, cncf:%s]",
		dbDev.Username, dbDev.Entity, ghDev.Entity, cncfDev.GetLatestAffiliation(),
		dbDev.Email, ghDev.Email, cncfDev.GetBestIdentity())
}

func GetDeveloper(db *sql.DB, username string) (*Developer, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	stmt, err := db.Prepare(selectDeveloperSQL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare developer select statement")
	}

	row := stmt.QueryRow(username)

	u := &Developer{}
	if err = row.Scan(&u.Username, &u.Updated, &u.ID, &u.FullName,
		&u.Email, &u.AvatarURL, &u.ProfileURL, &u.Entity, &u.Location); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to scan row")
	}

	return u, nil
}

func mapDeveloperListItem(rows *sql.Rows) ([]*DeveloperListItem, error) {
	list := make([]*DeveloperListItem, 0)
	for rows.Next() {
		u := &DeveloperListItem{}
		if err := rows.Scan(&u.Username, &u.Entity, &u.UpdatedDate); err != nil {
			if err == sql.ErrNoRows {
				return list, nil
			}
			return nil, errors.Wrap(err, "failed to scan row")
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
		return nil, errors.Wrap(err, "failed to prepare developer query statement")
	}

	val = fmt.Sprintf("%%%s%%", val)
	rows, err := stmt.Query(val, val, val, limit)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute select statement")
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
		return errors.Wrap(err, "failed to prepare entity update statement")
	}

	tx, err := db.Begin()
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}

	for username, name := range devs {
		if _, err = tx.Stmt(updateStmt).Exec(name, username); err != nil {
			rollbackTransaction(tx)
			return errors.Wrapf(err, "error updating full name for %s to %s", username, name)
		}
	}

	if err = tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}
