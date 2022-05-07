package data

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
)

func SaveAll(ids []int64) error {
	if db == nil {
		return errors.New("database not initialized")
	}

	stmt, err := db.Prepare("INSERT INTO sample (id, val) VALUES (?, ?)")
	if err != nil {
		return errors.Wrapf(err, "failed to prepare batch statement")
	}

	tx, err := db.Begin()
	if err != nil {
		return errors.Wrapf(err, "failed to begin transaction")
	}

	for _, id := range ids {
		val := fmt.Sprintf("%d", time.Now().UTC().Unix())
		_, err = tx.Stmt(stmt).Exec(id, val)
		if err != nil {
			if err = tx.Rollback(); err != nil {
				return errors.Wrapf(err, "failed to rollback transaction")
			}
			return errors.Wrapf(err, "failed to execute batch statement")
		}
	}

	if err = tx.Commit(); err != nil {
		return errors.Wrapf(err, "failed to commit transaction")
	}

	return nil
}
