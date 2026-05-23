// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"errors"

	"maand/bucket"
)

// UpdateAllocationPlan records staged content hash and target version for an allocation.
func UpdateAllocationPlan(tx *sql.Tx, namespace, key, hash, newVersion string) error {
	newVersion = NormalizeDeployVersion(newVersion)

	var storedCurrentHash string
	row := tx.QueryRow(
		`SELECT ifnull(current_hash, '') FROM hash WHERE namespace = ? AND key = ?`,
		namespace, key,
	)
	err := row.Scan(&storedCurrentHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, err = tx.Exec(
				`INSERT INTO hash (namespace, key, current_hash, current_version, new_version)
				 VALUES (?, ?, ?, ?, ?)`,
				namespace, key, hash, DefaultAllocationVersion, newVersion,
			)
			if err != nil {
				return bucket.DatabaseError(err)
			}
			return nil
		}
		return bucket.DatabaseError(err)
	}

	_, err = tx.Exec(
		`UPDATE hash SET current_hash = ?, new_version = ? WHERE namespace = ? AND key = ?`,
		hash, newVersion, namespace, key,
	)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

// PromoteAllocationState marks staged content and target version as live on the allocation.
func PromoteAllocationState(tx *sql.Tx, namespace, key string) error {
	_, err := tx.Exec(
		`UPDATE hash
		 SET previous_hash = current_hash,
		     current_version = ifnull(new_version, ?)
		 WHERE namespace = ? AND key = ?`,
		DefaultAllocationVersion, namespace, key,
	)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

// ClearAllocationLiveState clears promoted hash and running version for disabled allocations.
func ClearAllocationLiveState(tx *sql.Tx, namespace, key string) error {
	_, err := tx.Exec(
		`UPDATE hash SET previous_hash = NULL, current_version = NULL WHERE namespace = ? AND key = ?`,
		namespace, key,
	)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

// GetAllocationVersions returns running and target versions for an allocation hash row.
func GetAllocationVersions(tx *sql.Tx, namespace, key string) (AllocationVersions, error) {
	var currentVersion, newVersion sql.NullString
	row := tx.QueryRow(
		`SELECT current_version, new_version FROM hash WHERE namespace = ? AND key = ?`,
		namespace, key,
	)
	err := row.Scan(&currentVersion, &newVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AllocationVersions{
				CurrentVersion: DefaultAllocationVersion,
				NewVersion:     DefaultAllocationVersion,
			}, nil
		}
		return AllocationVersions{}, bucket.DatabaseError(err)
	}

	return AllocationVersions{
		CurrentVersion: normalizeStoredVersion(currentVersion),
		NewVersion:     normalizeStoredVersion(newVersion),
	}, nil
}
