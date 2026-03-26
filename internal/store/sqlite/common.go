package sqlite

import (
	"database/sql"

	"github.com/WAY29/SimplePool/internal/store"
)

func ensureRowsAffected(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return store.ErrNotFound
	}

	return nil
}
