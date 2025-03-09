// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gc

import (
	"fmt"
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

	deletes := []string{
		"DELETE FROM allocations WHERE removed = 1",
	}

	rows, err := tx.Query("SELECT job, alloc_id FROM allocations WHERE removed = 1")
	if err != nil {
		return data.NewDatabaseError(err)
	}

	for rows.Next() {
		var job string
		var allocID string
		if err := rows.Scan(&job, &allocID); err != nil {
			return data.NewDatabaseError(err)
		}
		deletes = append(deletes, fmt.Sprintf("DELETE FROM hash WHERE namespace = '%s' AND key = '%s'", fmt.Sprintf("%s_allocation", job), allocID))
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

	return nil
}
