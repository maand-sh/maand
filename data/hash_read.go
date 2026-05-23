// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"errors"

	"maand/bucket"
)

// GetAllocationHash returns staged and promoted hashes for an allocation.
// ok is false when no hash row exists yet.
func GetAllocationHash(tx *sql.Tx, namespace, key string) (current, previous string, ok bool, err error) {
	row := tx.QueryRow(
		"SELECT ifnull(current_hash, ''), ifnull(previous_hash, '') FROM hash WHERE namespace = ? AND key = ?",
		namespace, key,
	)
	err = row.Scan(&current, &previous)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", false, nil
		}
		return "", "", false, bucket.DatabaseError(err)
	}
	return current, previous, true, nil
}
