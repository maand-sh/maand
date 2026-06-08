// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"

	"maand/bucket"
)

// SetAllocationNewVersion records the deploy target version on an allocation row (build-time).
func SetAllocationNewVersion(tx *sql.Tx, allocID, version string) error {
	version = NormalizeDeployVersion(version)
	_, err := tx.Exec(`UPDATE allocations SET new_version = ? WHERE alloc_id = ?`, version, allocID)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

// GetAllocationNewVersion returns the target version stored on the allocation row.
func GetAllocationNewVersion(tx *sql.Tx, allocID string) (string, error) {
	var newVersion sql.NullString
	err := tx.QueryRow(`SELECT new_version FROM allocations WHERE alloc_id = ?`, allocID).Scan(&newVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			return DefaultAllocationVersion, nil
		}
		return "", bucket.DatabaseError(err)
	}
	return normalizeStoredVersion(newVersion), nil
}

// GetVersionPendingNonRemovedAllocations returns workers (active or disabled) whose running
// version differs from the build target.
func GetVersionPendingNonRemovedAllocations(tx *sql.Tx, job string) ([]string, error) {
	namespace := job + "_allocation"
	rows, err := tx.Query(`
		SELECT a.worker_ip
		FROM allocations a
		LEFT JOIN hash h ON h.namespace = ? AND h.key = a.alloc_id
		WHERE a.job = ? AND a.removed = 0
		  AND ifnull(h.current_version, ?) != ifnull(a.new_version, ?)`,
		namespace, job, DefaultAllocationVersion, DefaultAllocationVersion,
	)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	workers := make([]string, 0)
	for rows.Next() {
		var workerIP string
		if err := rows.Scan(&workerIP); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		workers = append(workers, workerIP)
	}
	if err := rowsErr(rows); err != nil {
		return nil, err
	}
	return workers, nil
}

// GetVersionPendingAllocations returns active workers whose running version differs from the build target.
func GetVersionPendingAllocations(tx *sql.Tx, job string) ([]string, error) {
	namespace := job + "_allocation"
	rows, err := tx.Query(`
		SELECT a.worker_ip
		FROM allocations a
		LEFT JOIN hash h ON h.namespace = ? AND h.key = a.alloc_id
		WHERE a.job = ? AND a.removed = 0 AND a.disabled = 0
		  AND ifnull(h.current_version, ?) != ifnull(a.new_version, ?)`,
		namespace, job, DefaultAllocationVersion, DefaultAllocationVersion,
	)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	workers := make([]string, 0)
	for rows.Next() {
		var workerIP string
		if err := rows.Scan(&workerIP); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		workers = append(workers, workerIP)
	}
	if err := rowsErr(rows); err != nil {
		return nil, err
	}
	return workers, nil
}

// AllocationNeedsVersionRollout reports whether running and target versions differ for one allocation.
func AllocationNeedsVersionRollout(tx *sql.Tx, job, workerIP string) (bool, error) {
	allocID, err := GetAllocationID(tx, workerIP, job)
	if err != nil {
		return false, err
	}
	namespace := job + "_allocation"
	versions, err := GetAllocationVersions(tx, namespace, allocID)
	if err != nil {
		return false, err
	}
	return versions.CurrentVersion != versions.NewVersion, nil
}
