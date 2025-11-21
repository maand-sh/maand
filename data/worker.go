// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"maand/bucket"
)

func GetWorkers(tx *sql.Tx, labels []string) ([]string, error) {
	workers := make([]string, 0)

	readRows := func(rows *sql.Rows) error {
		for rows.Next() {
			var ip string
			err := rows.Scan(&ip)
			if err != nil {
				return err
			}
			workers = append(workers, ip)
		}
		return rows.Err()
	}

	if len(labels) == 0 {
		rows, err := tx.Query("SELECT worker_ip FROM worker ORDER BY position")
		if err != nil {
			return workers, bucket.DatabaseError(err)
		}
		err = readRows(rows)
		if err != nil {
			return workers, bucket.DatabaseError(err)
		}
	}

	if len(labels) > 0 {
		query := fmt.Sprintf("SELECT DISTINCT worker_ip FROM worker w JOIN worker_labels wl ON w.worker_id = wl.worker_id WHERE label in ('%s') ORDER BY position", strings.Join(labels, `','`))
		rows, err := tx.Query(query)
		if err != nil {
			return workers, bucket.DatabaseError(err)
		}
		err = readRows(rows)
		if err != nil {
			return workers, bucket.DatabaseError(err)
		}
	}

	return workers, nil
}

func GetWorkerID(tx *sql.Tx, workerIP string) (string, error) {
	var workerID string
	row := tx.QueryRow("SELECT worker_id FROM worker WHERE worker_ip = ?", workerIP)
	err := row.Scan(&workerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", bucket.DatabaseError(err)
		}
	}
	return workerID, nil
}

func GetWorkerLabels(tx *sql.Tx, workerID string) ([]string, error) {
	rows, err := tx.Query("SELECT label FROM worker_labels WHERE worker_id = ? ORDER BY label", workerID)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	labels := make([]string, 0)
	for rows.Next() {
		var label string
		err := rows.Scan(&label)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}
		labels = append(labels, label)
	}
	return labels, nil
}

func GetLabels(tx *sql.Tx) ([]string, error) {
	rows, err := tx.Query("SELECT DISTINCT label FROM worker_labels ORDER BY label")
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	labels := make([]string, 0)
	for rows.Next() {
		var label string
		err := rows.Scan(&label)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}
		labels = append(labels, label)
	}
	return labels, nil
}

func GetWorkerTags(tx *sql.Tx, workerID string) (map[string]string, error) {
	rows, err := tx.Query("SELECT key, value FROM worker_tags WHERE worker_id = ? ORDER BY key", workerID)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	tags := make(map[string]string)
	for rows.Next() {
		var key, value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}
		tags[key] = value
	}
	return tags, nil
}

func GetAllWorkers(tx *sql.Tx) ([]string, error) {
	var workers []string
	rows, err := tx.Query("SELECT worker_ip FROM worker")
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	for rows.Next() {
		var workerIP string
		err := rows.Scan(&workerIP)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}
		workers = append(workers, workerIP)
	}
	return workers, nil
}

func GetWorkerCPU(tx *sql.Tx, workerIP string) (string, error) {
	var availableCPUMhz string
	row := tx.QueryRow("SELECT available_cpu_mhz FROM worker WHERE worker_ip = ?", workerIP)
	err := row.Scan(&availableCPUMhz)
	if err != nil {
		return "", bucket.DatabaseError(err)
	}
	return availableCPUMhz, nil
}

func GetWorkerMemory(tx *sql.Tx, workerIP string) (string, error) {
	var availableMemoryMb string
	row := tx.QueryRow("SELECT available_memory_mb FROM worker WHERE worker_ip = ?", workerIP)
	err := row.Scan(&availableMemoryMb)
	if err != nil {
		return "", bucket.DatabaseError(err)
	}
	return availableMemoryMb, nil
}

func GetAllocatedJobs(tx *sql.Tx, workerIP string) ([]string, error) {
	rows, err := tx.Query("SELECT job FROM allocations WHERE worker_ip = ?", workerIP)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	var allocatedJobs []string
	for rows.Next() {
		var job string
		err = rows.Scan(&job)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}
		allocatedJobs = append(allocatedJobs, job)
	}
	return allocatedJobs, nil
}

func GetAllocatedWorkers(tx *sql.Tx, job string) ([]string, error) {
	rows, err := tx.Query("SELECT worker_ip FROM allocations WHERE job = ?", job)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	var allocatedWorkers []string
	for rows.Next() {
		var workerIP string
		err = rows.Scan(&workerIP)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}
		allocatedWorkers = append(allocatedWorkers, workerIP)
	}
	return allocatedWorkers, nil
}

func IsAllocationDisabled(tx *sql.Tx, workerIP, job string) (int, error) {
	var disabled int
	row := tx.QueryRow("SELECT disabled FROM allocations WHERE worker_ip = ? AND job = ?", workerIP, job)
	err := row.Scan(&disabled)
	if err != nil {
		return -1, bucket.DatabaseError(err)
	}
	return disabled, nil
}

func IsAllocationRemoved(tx *sql.Tx, workerIP, job string) (int, error) {
	var removed int
	row := tx.QueryRow("SELECT removed FROM allocations WHERE worker_ip = ? AND job = ?", workerIP, job)
	err := row.Scan(&removed)
	if err != nil {
		return -1, bucket.DatabaseError(err)
	}
	return removed, nil
}

func GetActiveAllocations(tx *sql.Tx, job string) ([]string, error) {
	var activeWorkers []string
	workers, err := GetAllocatedWorkers(tx, job)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	for _, workerIP := range workers {
		removed, err := IsAllocationRemoved(tx, workerIP, job)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		if removed == 1 {
			continue
		}
		activeWorkers = append(activeWorkers, workerIP)
	}
	return activeWorkers, nil
}
