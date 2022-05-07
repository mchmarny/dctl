package data

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
)

func Update(id int64) (updated bool, err error) {
	if db == nil {
		return false, errors.New("database not initialized")
	}

	stmt, err := db.Prepare("UPDATE sample SET val = ? WHERE id = ?")
	if err != nil {
		return false, errors.Wrapf(err, "failed to prepare batch statement")
	}

	val := fmt.Sprintf("%d", time.Now().UTC().Unix())
	res, err := stmt.Exec(val, id)
	if err != nil {
		return false, errors.Wrapf(err, "failed to execute update statement")
	}

	affect, err := res.RowsAffected()
	if err != nil {
		return false, errors.Wrapf(err, "failed to get affected rows")
	}

	return affect > 0, nil
}
