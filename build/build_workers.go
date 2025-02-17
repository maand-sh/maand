package build

import (
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"maand/data"
	"maand/utils"
	"maand/workspace"
)

var removedWorkers []string

func Workers(tx *sql.Tx, ws *workspace.DefaultWorkspace) {
	var workspaceWorkers []string

	var workersIP []string
	for _, worker := range ws.GetWorkers() {
		workersIP = append(workersIP, worker.Host)
	}

	if len(workersIP) != len(utils.Unique(workersIP)) {
		panic("workers.json can't have duplicate worker ip")
	}

	for _, worker := range ws.GetWorkers() {
		row := tx.QueryRow("SELECT worker_id FROM worker WHERE worker_ip = ?", worker.Host)

		var workerID string
		_ = row.Scan(&workerID)
		if workerID == "" {
			workerID = uuid.NewString()
		}

		workspaceWorkers = append(workspaceWorkers, worker.Host)

		availableMemory, err := utils.ExtractSizeInMB(worker.Memory)
		utils.Check(err)
		if availableMemory < 0 {
			panic(fmt.Sprintf("worker memory can't be less than 0, worker %s", worker.Host))
		}

		availableCPU, err := utils.ExtractCPUFrequencyInMHz(worker.CPU)
		utils.Check(err)
		if availableCPU < 0 {
			panic(fmt.Sprintf("worker cpu can't be less than 0, worker %s", worker.Host))
		}

		query := "INSERT OR REPLACE INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position) VALUES (?, ?, ?, ?, ?)"
		_, err = tx.Exec(query, workerID, worker.Host, fmt.Sprintf("%v", availableMemory), fmt.Sprintf("%v", availableCPU), worker.Position)
		utils.Check(err)

		err = workerLabels(tx, workerID, worker)
		utils.Check(err)

		err = workerTags(tx, workerID, worker)
		utils.Check(err)
	}

	removedWorkers = []string{}
	availableWorkers := data.GetAllWorkers(tx)
	diffs := utils.Difference(availableWorkers, workspaceWorkers)
	for _, workerIP := range diffs {
		_, err := tx.Exec("DELETE FROM worker_labels WHERE worker_id IN (SELECT worker_id FROM worker WHERE worker_ip = ?)", workerIP)
		utils.Check(err)
		_, err = tx.Exec("DELETE FROM worker_tags WHERE worker_id IN (SELECT worker_id FROM worker WHERE worker_ip = ?)", workerIP)
		utils.Check(err)
		_, err = tx.Exec("DELETE FROM worker WHERE worker_ip = ?", workerIP)
		utils.Check(err)

		removedWorkers = append(removedWorkers, workerIP)
	}
}

func workerLabels(tx *sql.Tx, workerID string, worker workspace.Worker) error {
	_, err := tx.Exec("DELETE FROM worker_labels WHERE worker_id = ?", workerID)
	utils.Check(err)

	labels := worker.Labels

	if len(labels) != len(utils.Unique(labels)) {
		panic(fmt.Sprintf("workers can't have duplicate labels, worker: %s", worker.Host))
	}

	for _, label := range labels {
		_, err := tx.Exec("INSERT INTO worker_labels (worker_id, label) VALUES (?, ?)", workerID, label)
		utils.Check(err)
	}
	return nil
}

func workerTags(tx *sql.Tx, workerID string, worker workspace.Worker) error {
	_, err := tx.Exec("DELETE FROM worker_tags WHERE worker_id = ?", workerID)
	utils.Check(err)

	tags := worker.Tags
	for key, value := range tags {
		_, err = tx.Exec("INSERT INTO worker_tags (worker_id, key, value) VALUES (?, ?, ?)", workerID, key, value)
		utils.Check(err)
	}
	return nil
}
