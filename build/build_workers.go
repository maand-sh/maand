// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"errors"
	"fmt"

	"maand/bucket"
	"maand/data"
	"maand/utils"
	"maand/workspace"

	"github.com/google/uuid"
)

func BuildWorkers(tx *sql.Tx, jobWorkspace *workspace.DefaultWorkspace) ([]string, error) {
	var workspaceWorkerIPs []string

	workspaceWorkers, err := jobWorkspace.GetWorkers()
	if err != nil {
		return nil, err
	}

	workerIPs := make([]string, 0, len(workspaceWorkers))
	for _, worker := range workspaceWorkers {
		workerIPs = append(workerIPs, worker.Host)
	}

	if len(workerIPs) != len(utils.Unique(workerIPs)) {
		// TODO: report duplicate ip address
		return nil, fmt.Errorf("%w: duplicate worker ip found", bucket.ErrInvalidWorkerJSON)
	}

	for _, worker := range workspaceWorkers {
		row := tx.QueryRow("SELECT worker_id FROM worker WHERE worker_ip = ?", worker.Host)

		var workerID string
		err := row.Scan(&workerID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, bucket.DatabaseError(err)
		}
		if workerID == "" {
			workerID = uuid.NewString()
		}

		workspaceWorkerIPs = append(workspaceWorkerIPs, worker.Host)

		availableMemoryMB, err := utils.ExtractSizeInMB(worker.Memory)
		if err != nil {
			return nil, fmt.Errorf("%w: worker %s %w", bucket.ErrInvalidWorkerJSON, worker.Host, err)
		}
		if availableMemoryMB < 0 {
			return nil, fmt.Errorf("%w: worker %s memory can't be less than 0", bucket.ErrInvalidWorkerJSON, worker.Host)
		}

		availableCPUMHz, err := utils.ExtractCPUFrequencyInMHz(worker.CPU)
		if err != nil {
			return nil, fmt.Errorf("%w: worker %s %w", bucket.ErrInvalidWorkerJSON, worker.Host, err)
		}
		if availableCPUMHz < 0 {
			return nil, fmt.Errorf("%w: worker %s, cpu can't be less than 0", bucket.ErrInvalidWorkerJSON, worker.Host)
		}

		upsertWorkerQuery := "INSERT OR REPLACE INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position) VALUES (?, ?, ?, ?, ?)"
		_, err = tx.Exec(upsertWorkerQuery, workerID, worker.Host, fmt.Sprintf("%v", availableMemoryMB), fmt.Sprintf("%v", availableCPUMHz), worker.Position)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		if err := syncWorkerLabels(tx, workerID, worker); err != nil {
			return nil, err
		}

		if err := syncWorkerTags(tx, workerID, worker); err != nil {
			return nil, err
		}
	}

	databaseWorkerIPs, err := data.GetAllWorkers(tx)
	if err != nil {
		return nil, err
	}

	workersToRemove := utils.Difference(databaseWorkerIPs, workspaceWorkerIPs)
	removedWorkers := make([]string, 0, len(workersToRemove))
	for _, workerIP := range workersToRemove {
		_, err := tx.Exec("DELETE FROM worker_labels WHERE worker_id IN (SELECT worker_id FROM worker WHERE worker_ip = ?)", workerIP)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		_, err = tx.Exec("DELETE FROM worker_tags WHERE worker_id IN (SELECT worker_id FROM worker WHERE worker_ip = ?)", workerIP)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		_, err = tx.Exec("DELETE FROM worker WHERE worker_ip = ?", workerIP)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}
		removedWorkers = append(removedWorkers, workerIP)
	}
	return removedWorkers, nil
}

func syncWorkerLabels(tx *sql.Tx, workerID string, worker workspace.Worker) error {
	_, err := tx.Exec("DELETE FROM worker_labels WHERE worker_id = ?", workerID)
	if err != nil {
		return bucket.DatabaseError(err)
	}

	labels := worker.Labels
	if len(labels) != len(utils.Unique(labels)) {
		return fmt.Errorf("%w: worker %s have duplicate labels", bucket.ErrInvalidWorkerJSON, worker.Host)
	}

	for _, label := range labels {
		_, err := tx.Exec("INSERT INTO worker_labels (worker_id, label) VALUES (?, ?)", workerID, label)
		if err != nil {
			return bucket.DatabaseError(err)
		}
	}
	return nil
}

func syncWorkerTags(tx *sql.Tx, workerID string, worker workspace.Worker) error {
	_, err := tx.Exec("DELETE FROM worker_tags WHERE worker_id = ?", workerID)
	if err != nil {
		return bucket.DatabaseError(err)
	}

	tags := worker.Tags
	// TODO: P1, check for duplicate
	for key, value := range tags {
		_, err = tx.Exec("INSERT INTO worker_tags (worker_id, key, value) VALUES (?, ?, ?)", workerID, key, value)
		if err != nil {
			return bucket.DatabaseError(err)
		}
	}
	return nil
}
