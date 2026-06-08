// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"errors"
	"fmt"

	"maand/bucket"
)

// HealthFailedPreviousHash is stored in previous_hash to signal a health-check failure
// until the next successful deploy promotes the allocation again.
const HealthFailedPreviousHash = "__maand_health_failed__"

// MarkAllocationHealthFailed marks a promoted allocation as needing redeploy.
func MarkAllocationHealthFailed(tx *sql.Tx, job, workerIP string) error {
	namespace := fmt.Sprintf("%s_allocation", job)
	allocID, err := GetAllocationID(tx, workerIP, job)
	if err != nil {
		return err
	}

	var previousHash, currentHash sql.NullString
	err = tx.QueryRow(
		`SELECT previous_hash, current_hash FROM hash WHERE namespace = ? AND key = ?`,
		namespace, allocID,
	).Scan(&previousHash, &currentHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return bucket.DatabaseError(err)
	}
	if !previousHash.Valid || previousHash.String == "" {
		return nil
	}
	if !currentHash.Valid || currentHash.String == "" {
		return nil
	}
	if previousHash.String != currentHash.String {
		return nil
	}

	_, err = tx.Exec(
		`UPDATE hash SET previous_hash = ? WHERE namespace = ? AND key = ?`,
		HealthFailedPreviousHash, namespace, allocID,
	)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}
