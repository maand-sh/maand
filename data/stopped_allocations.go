// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"

	"maand/bucket"
)

// StoppedAllocation is an allocation marked removed or disabled in the catalog.
type StoppedAllocation struct {
	WorkerIP string
	Job      string
	Removed  bool
	Disabled bool
}

// ListStoppedAllocations returns allocations with removed = 1 or disabled = 1.
func ListStoppedAllocations(tx *sql.Tx) ([]StoppedAllocation, error) {
	rows, err := tx.Query(`
		SELECT worker_ip, job, removed, disabled
		FROM allocations
		WHERE removed = 1 OR disabled = 1`)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	allocs := make([]StoppedAllocation, 0)
	for rows.Next() {
		var alloc StoppedAllocation
		var removed, disabled int
		if err := rows.Scan(&alloc.WorkerIP, &alloc.Job, &removed, &disabled); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		alloc.Removed = removed == 1
		alloc.Disabled = disabled == 1
		allocs = append(allocs, alloc)
	}
	if err := rowsErr(rows); err != nil {
		return nil, err
	}
	return allocs, nil
}
