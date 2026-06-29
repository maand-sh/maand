// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"

	"maand/bucket"
)

// CountJobAllocations returns non-removed allocation rows for jobName.
// When activeOnly is true, disabled rows are excluded as well.
func CountJobAllocations(tx *sql.Tx, jobName string, activeOnly bool) (int, error) {
	query := `SELECT COUNT(1) FROM allocations WHERE job = ? AND removed = 0`
	if activeOnly {
		query += ` AND disabled = 0`
	}

	var count int
	if err := tx.QueryRow(query, jobName).Scan(&count); err != nil {
		return 0, bucket.DatabaseError(err)
	}
	return count, nil
}

// CountAllocations returns rows in allocations. When activeOnly is true, removed rows are excluded.
func CountAllocations(tx *sql.Tx, activeOnly bool) (int, error) {
	query := `SELECT COUNT(1) FROM allocations`
	if activeOnly {
		query += ` WHERE removed = 0`
	}

	var count int
	if err := tx.QueryRow(query).Scan(&count); err != nil {
		return 0, bucket.DatabaseError(err)
	}
	return count, nil
}
