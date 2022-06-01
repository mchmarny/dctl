package data

import (
	"database/sql"
	"fmt"

	"github.com/pkg/errors"
)

const (
	insertSubSQL = `INSERT INTO sub (type, old, new) VALUES (?, ?, ?) 
		ON CONFLICT(type, old) DO UPDATE SET new = ?
	`

	selectSubSQL = `SELECT type, old, new FROM sub`
)

var (
	UpdatableProperties = []string{
		"entity",
	}

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
		"LC":          true,
		"P.C.":        true,
		"P.A.":        true,
		"S.C.":        true,
		"LTD.":        true,
		"CHTD.":       true,
		"PC":          true,
		"LTD":         true,
		"PVT":         true,
		"SE":          true,
		"S.A.":        true,
	}

	entitySubstitutions = map[string]string{
		"CHAINGUARDDEV":       "CHAINGUARD",
		"GCP":                 "GOOGLE",
		"GOOGLECLOUD":         "GOOGLE",
		"GOOGLECLOUDPLATFORM": "GOOGLE",
		"HUAWEICLOUD":         "HUAWEI",
		"IBM CODAITY":         "IBM",
		"IBM RESEARCH":        "IBM",
		"INTERNATIONAL BUSINESS MACHINES CORPORATION":                 "IBM",
		"INTERNATIONAL BUSINESS MACHINES":                             "IBM",
		"INTERNATIONAL INSTITUTE OF INFORMATION TECHNOLOGY BANGALORE": "IIIT BANGALORE",
		"LINE PLUS":       "LINE",
		"MICROSOFT CHINA": "MICROSOFT",
		"REDHATOFFICIAL":  "REDHAT",
		"S&P GLOBAL INC":  "S&P",
		"S&P GLOBAL":      "S&P",
		"VERVERICA ORIGINAL CREATORS OF APACHE FLINK": "VERVERICA",
	}
)

type Substitution struct {
	Prop    string `json:"prop"`
	Old     string `json:"old"`
	New     string `json:"new"`
	Records int64  `json:"records"`
}

func applyDeveloperSub(db *sql.DB, sub *Substitution) error {
	if db == nil {
		return errDBNotInitialized
	}

	if sub == nil {
		return nil
	}

	// CHeck if contains
	if !Contains(UpdatableProperties, sub.Prop) {
		return errors.Errorf("invalid property: %s (permitted options: %v)", sub.Prop, UpdatableProperties)
	}

	stmt, err := db.Prepare(fmt.Sprintf(updateDeveloperPropertySQL, sub.Prop, sub.Prop))
	if err != nil {
		return errors.Wrap(err, "failed to prepare sql statement")
	}

	res, err := stmt.Exec(sub.New, sub.Old)
	if err != nil && err != sql.ErrNoRows {
		return errors.Wrap(err, "failed to execute developer property update statement")
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	sub.Records = rows

	return nil
}

func SaveAndApplyDeveloperSub(db *sql.DB, prop, old, new string) (*Substitution, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	s := &Substitution{
		Prop: prop,
		Old:  old,
		New:  new,
	}

	if err := applyDeveloperSub(db, s); err != nil {
		return nil, errors.Wrap(err, "failed to apply developer sub")
	}

	subStmt, err := db.Prepare(insertSubSQL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to prepare state insert statement")
	}

	if _, err = subStmt.Exec(prop, old, new, new); err != nil {
		return nil, errors.Wrap(err, "failed to insert state")
	}

	return s, nil
}

func ApplySubstitutions(db *sql.DB) ([]*Substitution, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	stmt, err := db.Prepare(selectSubSQL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare sql statement")
	}

	rows, err := stmt.Query()
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute substitute select statement")
	}
	defer rows.Close()

	list := make([]*Substitution, 0)
	for rows.Next() {
		s := &Substitution{}
		if err := rows.Scan(&s.Prop, &s.Old, &s.New); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
		}
		list = append(list, s)
	}

	for _, s := range list {
		if err := applyDeveloperSub(db, s); err != nil {
			return nil, errors.Wrap(err, "failed to apply developer sub")
		}
	}

	return list, nil
}
