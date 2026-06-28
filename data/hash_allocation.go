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

// RemoveAllocationHash deletes the deploy plan hash row for an allocation.
func RemoveAllocationHash(tx *sql.Tx, job, allocID string) error {
	namespace := fmt.Sprintf("%s_allocation", job)
	return RemoveHash(tx, namespace, allocID)
}

// UpdateAllocationPlan records staged content hash and file manifest for an allocation.
// Target version lives on the allocations row (updated by build).
func UpdateAllocationPlan(tx *sql.Tx, namespace, key, hash string, currentFiles FileManifest) error {
	encodedFiles, err := currentFiles.Encode()
	if err != nil {
		return err
	}

	var storedCurrentHash string
	row := tx.QueryRow(
		`SELECT ifnull(current_hash, '') FROM hash WHERE namespace = ? AND key = ?`,
		namespace, key,
	)
	err = row.Scan(&storedCurrentHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, err = tx.Exec(
				`INSERT INTO hash (namespace, key, current_hash, current_version, current_files)
				 VALUES (?, ?, ?, ?, ?)`,
				namespace, key, hash, DefaultAllocationVersion, encodedFiles,
			)
			if err != nil {
				return bucket.DatabaseError(err)
			}
			return nil
		}
		return bucket.DatabaseError(err)
	}

	_, err = tx.Exec(
		`UPDATE hash SET current_hash = ?, current_files = ? WHERE namespace = ? AND key = ?`,
		hash, encodedFiles, namespace, key,
	)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

// PromoteAllocationState marks staged content and running version as live on the allocation.
func PromoteAllocationState(tx *sql.Tx, namespace, key string) error {
	_, err := tx.Exec(
		`UPDATE hash
		 SET previous_hash = current_hash,
		     previous_files = current_files,
		     current_version = COALESCE(
		       (SELECT new_version FROM allocations WHERE alloc_id = ?),
		       ?
		     )
		 WHERE namespace = ? AND key = ?`,
		key, DefaultAllocationVersion, namespace, key,
	)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

// MarkAllocationStartPending clears previous_hash so deploy treats a re-enabled allocation as needing start.
func MarkAllocationStartPending(tx *sql.Tx, job, allocID string) error {
	namespace := fmt.Sprintf("%s_allocation", job)
	_, err := tx.Exec(
		`UPDATE hash SET previous_hash = NULL, previous_files = NULL WHERE namespace = ? AND key = ?`,
		namespace, allocID,
	)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

// ClearAllocationLiveState clears promoted hash and running version for disabled allocations.
func ClearAllocationLiveState(tx *sql.Tx, namespace, key string) error {
	_, err := tx.Exec(
		`UPDATE hash SET previous_hash = NULL, previous_files = NULL, current_version = NULL WHERE namespace = ? AND key = ?`,
		namespace, key,
	)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

// GetAllocationVersions returns running version from hash and target version from allocations.
func GetAllocationVersions(tx *sql.Tx, namespace, key string) (AllocationVersions, error) {
	var currentVersion sql.NullString
	row := tx.QueryRow(
		`SELECT ifnull(h.current_version, '') FROM hash h WHERE h.namespace = ? AND h.key = ?`,
		namespace, key,
	)
	err := row.Scan(&currentVersion)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return AllocationVersions{}, bucket.DatabaseError(err)
	}

	newVersion, err := GetAllocationNewVersion(tx, key)
	if err != nil {
		return AllocationVersions{}, err
	}

	return AllocationVersions{
		CurrentVersion: normalizeStoredVersion(currentVersion),
		NewVersion:     newVersion,
	}, nil
}
