package data

import (
	"database/sql"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func Get(id int64) (val *string, err error) {
	if db == nil {
		return nil, errors.New("database not initialized")
	}

	stmt, err := db.Prepare("SELECT val FROM sample WHERE id = ?")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to prepare select statement")
	}

	row := stmt.QueryRow(id)

	var v string
	err = row.Scan(&v)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Debug().Err(err).Msgf("failed to find record for id: %d", id)
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to scan row")
	}

	return &v, nil
}
