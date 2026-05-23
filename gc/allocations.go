// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gc

import (
	"database/sql"
	"fmt"

	"maand/bucket"
)

type removedAllocation struct {
	WorkerIP string
	Job      string
}

func listRemovedAllocations(tx *sql.Tx) ([]removedAllocation, error) {
	rows, err := tx.Query(`SELECT worker_ip, job FROM allocations WHERE removed = 1`)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	allocs := make([]removedAllocation, 0)
	for rows.Next() {
		var alloc removedAllocation
		if err := rows.Scan(&alloc.WorkerIP, &alloc.Job); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		allocs = append(allocs, alloc)
	}
	if err := rows.Err(); err != nil {
		return nil, bucket.DatabaseError(err)
	}
	return allocs, nil
}

func purgeRemovedAllocations(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT job, alloc_id FROM allocations WHERE removed = 1`)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var jobName, allocationID string
		if err := rows.Scan(&jobName, &allocationID); err != nil {
			return bucket.DatabaseError(err)
		}

		namespace := fmt.Sprintf("%s_allocation", jobName)
		if _, err := tx.Exec(`DELETE FROM hash WHERE namespace = ? AND key = ?`, namespace, allocationID); err != nil {
			return bucket.DatabaseError(err)
		}
	}

	if err := rows.Err(); err != nil {
		return bucket.DatabaseError(err)
	}

	_, err = tx.Exec(`DELETE FROM allocations WHERE removed = 1`)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}
