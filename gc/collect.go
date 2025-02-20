// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gc

import (
	"maand/data"
	"maand/kv"
)

func Collect() error {
	db, err := data.GetDatabase(true)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	tx, err := db.Begin()
	if err != nil {
		return data.NewDatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// TODO : clean allocation hash first
	deletes := []string{
		"DELETE FROM allocations WHERE removed = 1",
	}

	for _, query := range deletes {
		_, err := tx.Exec(query)
		if err != nil {
			return data.NewDatabaseError(err)
		}
	}

	err = kv.GetKVStore().GC(tx, -1)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return data.NewDatabaseError(err)
	}

	if err = data.UpdateJournalModeDefault(db); err != nil {
		return err
	}

	return nil
}
