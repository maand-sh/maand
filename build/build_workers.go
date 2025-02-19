package build

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"maand/data"
	"maand/utils"
	"maand/workspace"
)

var removedWorkers []string

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
		return fmt.Errorf("workers.json can't have duplicate worker ip")
	}

	for _, worker := range wsWorkers {
		row := tx.QueryRow("SELECT worker_id FROM worker WHERE worker_ip = ?", worker.Host)

		var workerID string
		err := row.Scan(&workerID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if workerID == "" {
			workerID = uuid.NewString()
		}

		workspaceWorkers = append(workspaceWorkers, worker.Host)

		availableMemory, err := utils.ExtractSizeInMB(worker.Memory)
		if err != nil {
			return err
		}
		if availableMemory < 0 {
			return fmt.Errorf("worker memory can't be less than 0, worker %s", worker.Host)
		}

		availableCPU, err := utils.ExtractCPUFrequencyInMHz(worker.CPU)
		if err != nil {
			return err
		}
		if availableCPU < 0 {
			return fmt.Errorf("worker cpu can't be less than 0, worker %s", worker.Host)
		}

		query := "INSERT OR REPLACE INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position) VALUES (?, ?, ?, ?, ?)"
		_, err = tx.Exec(query, workerID, worker.Host, fmt.Sprintf("%v", availableMemory), fmt.Sprintf("%v", availableCPU), worker.Position)
		if err != nil {
			return data.NewDatabaseError(err)
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
			return data.NewDatabaseError(err)
		}

		_, err = tx.Exec("DELETE FROM worker_tags WHERE worker_id IN (SELECT worker_id FROM worker WHERE worker_ip = ?)", workerIP)
		if err != nil {
			return data.NewDatabaseError(err)
		}

		_, err = tx.Exec("DELETE FROM worker WHERE worker_ip = ?", workerIP)
		if err != nil {
			return data.NewDatabaseError(err)
		}
		removedWorkers = append(removedWorkers, workerIP)
	}
	return nil
}

func workerLabels(tx *sql.Tx, workerID string, worker workspace.Worker) error {
	_, err := tx.Exec("DELETE FROM worker_labels WHERE worker_id = ?", workerID)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	labels := worker.Labels
	if len(labels) != len(utils.Unique(labels)) {
		return fmt.Errorf("workers can't have duplicate labels, worker: %s", worker.Host)
	}

	for _, label := range labels {
		_, err := tx.Exec("INSERT INTO worker_labels (worker_id, label) VALUES (?, ?)", workerID, label)
		if err != nil {
			return data.NewDatabaseError(err)
		}
	}
	return nil
}

func workerTags(tx *sql.Tx, workerID string, worker workspace.Worker) error {
	_, err := tx.Exec("DELETE FROM worker_tags WHERE worker_id = ?", workerID)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	tags := worker.Tags
	for key, value := range tags {
		_, err = tx.Exec("INSERT INTO worker_tags (worker_id, key, value) VALUES (?, ?, ?)", workerID, key, value)
		if err != nil {
			return data.NewDatabaseError(err)
		}
	}
	return nil
}
