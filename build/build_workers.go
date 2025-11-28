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

var removedWorkers []string // global variable captures removed workers from workspace

func Workers(tx *sql.Tx, ws *workspace.DefaultWorkspace) error {
	var workspaceWorkers []string

	wsWorkers, err := ws.GetWorkers()
	if err != nil {
		return err
	}

	var workersIP []string
	for _, worker := range wsWorkers {
		workersIP = append(workersIP, worker.Host)
	}

	if len(workersIP) != len(utils.Unique(workersIP)) {
		// TODO: report duplicate ip address
		return fmt.Errorf("%w: duplicate worker ip found", bucket.ErrInvaildWorkerJSON)
	}

	for _, worker := range wsWorkers {
		row := tx.QueryRow("SELECT worker_id FROM worker WHERE worker_ip = ?", worker.Host)

		var workerID string
		err := row.Scan(&workerID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return bucket.DatabaseError(err)
		}
		if workerID == "" {
			workerID = uuid.NewString()
		}

		workspaceWorkers = append(workspaceWorkers, worker.Host)

		availableMemory, err := utils.ExtractSizeInMB(worker.Memory)
		if err != nil {
			return fmt.Errorf("%w: worker %s %w", bucket.ErrInvaildWorkerJSON, worker.Host, err)
		}
		if availableMemory < 0 {
			return fmt.Errorf("%w: worker %s memory can't be less than 0", bucket.ErrInvaildWorkerJSON, worker.Host)
		}

		availableCPU, err := utils.ExtractCPUFrequencyInMHz(worker.CPU)
		if err != nil {
			return fmt.Errorf("%w: worker %s %w", bucket.ErrInvaildWorkerJSON, worker.Host, err)
		}
		if availableCPU < 0 {
			return fmt.Errorf("%w: worker %s, cpu can't be less than 0", bucket.ErrInvaildWorkerJSON, worker.Host)
		}

		query := "INSERT OR REPLACE INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position) VALUES (?, ?, ?, ?, ?)"
		_, err = tx.Exec(query, workerID, worker.Host, fmt.Sprintf("%v", availableMemory), fmt.Sprintf("%v", availableCPU), worker.Position)
		if err != nil {
			return bucket.DatabaseError(err)
		}

		err = workerLabels(tx, workerID, worker)
		if err != nil {
			return err
		}

		err = workerTags(tx, workerID, worker)
		if err != nil {
			return err
		}
	}

	removedWorkers = []string{}

	availableWorkers, err := data.GetAllWorkers(tx)
	if err != nil {
		return err
	}

	diffs := utils.Difference(availableWorkers, workspaceWorkers)
	for _, workerIP := range diffs {
		_, err := tx.Exec("DELETE FROM worker_labels WHERE worker_id IN (SELECT worker_id FROM worker WHERE worker_ip = ?)", workerIP)
		if err != nil {
			return bucket.DatabaseError(err)
		}

		_, err = tx.Exec("DELETE FROM worker_tags WHERE worker_id IN (SELECT worker_id FROM worker WHERE worker_ip = ?)", workerIP)
		if err != nil {
			return bucket.DatabaseError(err)
		}

		_, err = tx.Exec("DELETE FROM worker WHERE worker_ip = ?", workerIP)
		if err != nil {
			return bucket.DatabaseError(err)
		}
		removedWorkers = append(removedWorkers, workerIP)
	}
	return nil
}

func workerLabels(tx *sql.Tx, workerID string, worker workspace.Worker) error {
	_, err := tx.Exec("DELETE FROM worker_labels WHERE worker_id = ?", workerID)
	if err != nil {
		return bucket.DatabaseError(err)
	}

	labels := worker.Labels
	if len(labels) != len(utils.Unique(labels)) {
		return fmt.Errorf("%w: worker %s have duplicate labels", bucket.ErrInvaildWorkerJSON, worker.Host)
	}

	for _, label := range labels {
		_, err := tx.Exec("INSERT INTO worker_labels (worker_id, label) VALUES (?, ?)", workerID, label)
		if err != nil {
			return bucket.DatabaseError(err)
		}
	}
	return nil
}

func workerTags(tx *sql.Tx, workerID string, worker workspace.Worker) error {
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
