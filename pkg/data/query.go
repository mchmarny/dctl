package data

import (
	"database/sql"

	"github.com/pkg/errors"
)

func Query(minID int64) ([]string, error) {
	if db == nil {
		return nil, errors.New("database not initialized")
	}

	stmt, err := db.Prepare("SELECT val FROM sample WHERE id >= ?")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to prepare select statement")
	}

	rows, err := stmt.Query(minID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrapf(err, "failed to execute select statement")
	}
	defer rows.Close()

	list := make([]string, 0)
	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			return nil, errors.Wrapf(err, "failed to scan row")
		}
		list = append(list, val)
	}

	return list, nil
}
