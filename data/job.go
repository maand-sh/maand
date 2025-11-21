// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"fmt"
	"os"
	"path"

	"maand/bucket"
)

func GetJobs(tx *sql.Tx) ([]string, error) {
	rows, err := tx.Query("SELECT name FROM job")
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	jobs := make([]string, 0)
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		jobs = append(jobs, name)
	}
	return jobs, nil
}

func GetJobMemoryLimits(tx *sql.Tx, job string) (string, string, error) {
	var minMemory, maxMemory string
	row := tx.QueryRow("SELECT min_memory_mb, max_memory_mb FROM job WHERE name = ?", job)
	err := row.Scan(&minMemory, &maxMemory)
	if err != nil {
		return "", "", bucket.DatabaseError(err)
	}
	return minMemory, maxMemory, nil
}

func GetJobMemory(tx *sql.Tx, job string) (string, error) {
	var memory string
	row := tx.QueryRow("SELECT current_memory_mb FROM job WHERE name = ?", job)
	err := row.Scan(&memory)
	if err != nil {
		return "", bucket.DatabaseError(err)
	}
	return memory, nil
}

func GetJobCPULimits(tx *sql.Tx, job string) (string, string, error) {
	var minCPU, maxCPU string
	row := tx.QueryRow("SELECT min_cpu_mhz, max_cpu_mhz FROM job WHERE name = ?", job)
	err := row.Scan(&minCPU, &maxCPU)
	if err != nil {
		return "", "", bucket.DatabaseError(err)
	}
	return minCPU, maxCPU, nil
}

func GetJobCPU(tx *sql.Tx, job string) (string, error) {
	var cpu string
	row := tx.QueryRow("SELECT current_cpu_mhz FROM job WHERE name = ?", job)
	err := row.Scan(&cpu)
	if err != nil {
		return "", bucket.DatabaseError(err)
	}
	return cpu, nil
}

func GetJobVersion(tx *sql.Tx, job string) (string, error) {
	var version string
	row := tx.QueryRow("SELECT version FROM job WHERE name = ?", job)
	err := row.Scan(&version)
	if err != nil {
		return "", bucket.DatabaseError(err)
	}
	return version, nil
}

func GetJobSelectors(tx *sql.Tx, job string) ([]string, error) {
	rows, err := tx.Query("SELECT selector FROM job_selectors WHERE job_id = (SELECT job_id FROM job WHERE name = ?)", job)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	selectors := make([]string, 0)
	for rows.Next() {
		var selector string
		err := rows.Scan(&selector)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		selectors = append(selectors, selector)
	}
	return selectors, nil
}

func CopyJobFiles(tx *sql.Tx, job string, outputPath string) error {
	rows, err := tx.Query("SELECT path, content, isdir FROM job_files WHERE job_id = (SELECT job_id FROM job WHERE name = ?) ORDER BY isdir DESC", job)
	if err != nil {
		return bucket.DatabaseError(err)
	}

	for rows.Next() {
		var filePath string
		var content string
		var isDir bool
		err := rows.Scan(&filePath, &content, &isDir)
		if err != nil {
			return bucket.DatabaseError(err)
		}

		if isDir {
			err = os.MkdirAll(path.Join(outputPath, filePath), os.ModePerm)
			if err != nil {
				return bucket.DatabaseError(err)
			}
			continue
		}
		err = os.WriteFile(path.Join(outputPath, filePath), []byte(content), os.ModePerm)
		if err != nil {
			return bucket.DatabaseError(err)
		}
	}

	return nil
}

func GetAllocationID(tx *sql.Tx, workerIP, job string) (string, error) {
	var allocID string
	row := tx.QueryRow("SELECT alloc_id FROM allocations WHERE job = ? AND worker_ip = ?", job, workerIP)
	err := row.Scan(&allocID)
	if err != nil {
		return "", bucket.DatabaseError(err)
	}
	return allocID, nil
}

func GetNewAllocations(tx *sql.Tx, job string) ([]string, error) {
	rows, err := tx.Query("SELECT a.worker_ip FROM hash h JOIN allocations a ON h.namespace = ? AND h.key = a.alloc_id AND h.previous_hash is NULL WHERE a.job = ? AND a.removed = 0 AND a.disabled = 0", fmt.Sprintf("%s_allocation", job), job)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	allocations := make([]string, 0)
	for rows.Next() {
		var allocID string
		err := rows.Scan(&allocID)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		allocations = append(allocations, allocID)
	}
	return allocations, nil
}

func GetUpdatedAllocations(tx *sql.Tx, job string) ([]string, error) {
	rows, err := tx.Query("SELECT a.worker_ip FROM hash h JOIN allocations a ON h.namespace = ? AND h.key = a.alloc_id AND h.previous_hash IS NOT NULL AND h.previous_hash != h.current_hash WHERE a.job = ? AND a.removed = 0", fmt.Sprintf("%s_allocation", job), job)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	allocations := make([]string, 0)
	for rows.Next() {
		var allocID string
		err := rows.Scan(&allocID)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		allocations = append(allocations, allocID)
	}
	return allocations, nil
}

func GetJobCommands(tx *sql.Tx, job, event string) ([]string, error) {
	rows, err := tx.Query("SELECT DISTINCT name as command_name FROM job_commands WHERE job = ? AND executed_on = ?", job, event)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	commands := make([]string, 0)
	for rows.Next() {
		var command string
		err := rows.Scan(&command)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		commands = append(commands, command)
	}
	return commands, nil
}

func GetUpdateParallelCount(tx *sql.Tx, job string) (int, error) {
	var updateParallelCount int
	row := tx.QueryRow("SELECT update_parallel_count FROM job WHERE name = ?", job)
	err := row.Scan(&updateParallelCount)
	if err != nil {
		return 0, bucket.DatabaseError(err)
	}
	return updateParallelCount, nil
}

func GetJobsByDeploymentSeq(tx *sql.Tx, deploymentSeq int) ([]string, error) {
	var jobs []string
	rows, err := tx.Query("SELECT DISTINCT job FROM allocations WHERE deployment_seq = ?", deploymentSeq)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	for rows.Next() {
		var job string
		err = rows.Scan(&job)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		jobs = append(jobs, job)
	}
	return jobs, nil
}

func GetAllAllocatedJobs(tx *sql.Tx) ([]string, error) {
	rows, err := tx.Query("SELECT DISTINCT job FROM allocations")
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}

	jobs := make([]string, 0)
	for rows.Next() {
		var job string
		err := rows.Scan(&job)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		jobs = append(jobs, job)
	}
	return jobs, nil
}
