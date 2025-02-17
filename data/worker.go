package data

import (
	"database/sql"
	"errors"
	"fmt"
	"maand/utils"
	"strings"
)

func GetWorkers(tx *sql.Tx, labels []string) []string {
	workers := make([]string, 0)

	readRows := func(rows *sql.Rows) {
		for rows.Next() {
			var ip string
			err := rows.Scan(&ip)
			utils.Check(err)
			workers = append(workers, ip)
		}
	}

	if len(labels) == 0 {
		rows, err := tx.Query("SELECT worker_ip FROM worker ORDER BY position")
		utils.Check(err)
		readRows(rows)
	}

	if len(labels) > 0 {
		query := fmt.Sprintf("SELECT DISTINCT worker_ip FROM worker w JOIN worker_labels wl ON w.worker_id = wl.worker_id WHERE label in ('%s') ORDER BY position", strings.Join(labels, `','`))
		rows, err := tx.Query(query)
		utils.Check(err)
		readRows(rows)
	}

	return workers
}

func GetWorkerID(tx *sql.Tx, workerIP string) string {
	var workerID string
	row := tx.QueryRow("SELECT worker_id FROM worker WHERE worker_ip = ?", workerIP)
	err := row.Scan(&workerID)
	utils.Check(err)
	return workerID
}

func GetWorkerLabels(tx *sql.Tx, workerID string) []string {
	rows, err := tx.Query("SELECT label FROM worker_labels WHERE worker_id = ? ORDER BY label", workerID)
	utils.Check(err)

	labels := make([]string, 0)
	for rows.Next() {
		var label string
		err := rows.Scan(&label)
		utils.Check(err)
		labels = append(labels, label)
	}
	return labels
}

func GetLabels(tx *sql.Tx) []string {
	rows, err := tx.Query("SELECT DISTINCT label FROM worker_labels ORDER BY label")
	utils.Check(err)
	labels := make([]string, 0)
	for rows.Next() {
		var label string
		err := rows.Scan(&label)
		utils.Check(err)
		labels = append(labels, label)
	}
	return labels
}

func GetWorkerTags(tx *sql.Tx, workerID string) map[string]string {
	rows, err := tx.Query("SELECT key, value FROM worker_tags WHERE worker_id = ? ORDER BY key", workerID)
	utils.Check(err)
	tags := make(map[string]string)
	for rows.Next() {
		var key, value string
		err := rows.Scan(&key, &value)
		utils.Check(err)
		tags[key] = value
	}
	return tags
}

func GetAllWorkers(tx *sql.Tx) []string {
	var workers []string
	rows, err := tx.Query("SELECT worker_ip FROM worker")
	utils.Check(err)

	for rows.Next() {
		var workerIP string
		err := rows.Scan(&workerIP)
		utils.Check(err)
		workers = append(workers, workerIP)
	}
	return workers
}

func GetWorkerCPU(tx *sql.Tx, workerIP string) string {
	var availableCPUMhz string

	row := tx.QueryRow("SELECT available_cpu_mhz FROM worker WHERE worker_ip = ?", workerIP)
	err := row.Scan(&availableCPUMhz)
	utils.Check(err)

	return availableCPUMhz
}

func GetWorkerMemory(tx *sql.Tx, workerIP string) string {
	var availableMemoryMb string

	row := tx.QueryRow("SELECT available_memory_mb FROM worker WHERE worker_ip = ?", workerIP)
	err := row.Scan(&availableMemoryMb)
	utils.Check(err)

	return availableMemoryMb
}

func GetAllocatedJobs(tx *sql.Tx, workerIP string) []string {
	rows, err := tx.Query("SELECT job FROM allocations WHERE worker_ip = ?", workerIP)
	utils.Check(err)

	var allocatedJobs []string
	for rows.Next() {
		var job string
		err = rows.Scan(&job)
		utils.Check(err)
		allocatedJobs = append(allocatedJobs, job)
	}
	return allocatedJobs
}

func GetAllocatedWorkers(tx *sql.Tx, job string) []string {
	rows, err := tx.Query("SELECT worker_ip FROM allocations WHERE job = ?", job)
	utils.Check(err)

	var allocatedWorkers []string
	for rows.Next() {
		var workerIP string
		err = rows.Scan(&workerIP)
		utils.Check(err)

		allocatedWorkers = append(allocatedWorkers, workerIP)
	}
	return allocatedWorkers
}

func IsAllocationDisabled(tx *sql.Tx, workerIP, job string) int {
	var disabled int
	row := tx.QueryRow("SELECT disabled FROM allocations WHERE worker_ip = ? AND job = ?", workerIP, job)
	err := row.Scan(&disabled)
	if errors.Is(err, sql.ErrNoRows) {
		return 0
	}
	utils.Check(err)
	return disabled
}

func IsAllocationRemoved(tx *sql.Tx, workerIP, job string) int {
	var removed int
	row := tx.QueryRow("SELECT removed FROM allocations WHERE worker_ip = ? AND job = ?", workerIP, job)
	err := row.Scan(&removed)
	if errors.Is(err, sql.ErrNoRows) {
		return 0
	}
	utils.Check(err)
	return removed
}

func GetActiveAllocations(tx *sql.Tx, job string) []string {
	var activeWorkers []string
	workers := GetAllocatedWorkers(tx, job)
	for _, workerIP := range workers {
		if IsAllocationRemoved(tx, workerIP, job) == 1 {
			continue
		}
		activeWorkers = append(activeWorkers, workerIP)
	}
	return activeWorkers
}
