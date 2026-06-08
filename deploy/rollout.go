// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"

	"maand/bucket"
	"maand/data"
)

// JobNeedsRollout reports whether any active allocation for the job still needs
// a deploy wave (new worker or staged content not yet promoted to workers).
// After a successful deploy, promoteAllocationHash sets previous_hash = current_hash,
// so a re-run of maand deploy skips that job and continues with jobs that failed
// or have pending hash changes.
func JobNeedsRollout(tx *sql.Tx, job string) (bool, error) {
	workers, err := data.GetNonRemovedAllocations(tx, job)
	if err != nil {
		return false, err
	}
	namespace := fmt.Sprintf("%s_allocation", job)
	for _, workerIP := range workers {
		allocID, err := data.GetAllocationID(tx, workerIP, job)
		if err != nil {
			return false, err
		}
		var hashCount int
		err = tx.QueryRow(
			`SELECT count(*) FROM hash WHERE namespace = ? AND key = ?`,
			namespace, allocID,
		).Scan(&hashCount)
		if err != nil {
			return false, bucket.DatabaseError(err)
		}
		if hashCount == 0 {
			return true, nil
		}
	}

	newAllocations, err := data.GetNewAllocations(tx, job)
	if err != nil {
		return false, err
	}
	if len(newAllocations) > 0 {
		return true, nil
	}
	updatedAllocations, err := data.GetUpdatedNonRemovedAllocations(tx, job)
	if err != nil {
		return false, err
	}
	if len(updatedAllocations) > 0 {
		return true, nil
	}

	versionPending, err := data.GetVersionPendingNonRemovedAllocations(tx, job)
	if err != nil {
		return false, err
	}
	return len(versionPending) > 0, nil
}
