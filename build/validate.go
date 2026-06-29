package build

import (
	"database/sql"
	"fmt"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/workspace"
)

func ValidateWorkerResources(tx *sql.Tx) error {
	rows, err := tx.Query(`
        SELECT
            a.worker_ip, w.available_memory_mb, w.available_cpu_mhz, sum(j.current_memory_mb) as required_memory_mb, sum(j.current_cpu_mhz) AS required_cpu_mhz
        FROM
            allocations a JOIN job j ON j.name = a.job
                JOIN
            worker w ON w.worker_ip = a.worker_ip
        WHERE a.removed = 0 AND a.disabled = 0
        GROUP BY a.worker_ip
    `)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

		var resourceErrors []string
	for rows.Next() {
		var workerIP string
		var availableMemoryMB, availableCPUMHz, requiredMemoryMB, requiredCPUMHz float64
		err = rows.Scan(&workerIP, &availableMemoryMB, &availableCPUMHz, &requiredMemoryMB, &requiredCPUMHz)
		if err != nil {
			return bucket.DatabaseError(err)
		}

		if requiredMemoryMB > 0 && availableMemoryMB <= 0 {
			resourceErrors = append(resourceErrors, fmt.Sprintf(
				"worker_ip %s must specify memory in workers.json (allocated jobs require %.2f MB)",
				workerIP, requiredMemoryMB,
			))
		} else if requiredMemoryMB > 0 && availableMemoryMB < requiredMemoryMB {
			resourceErrors = append(resourceErrors, fmt.Sprintf(
				"worker_ip %s, available memory is %.2f MB, required memory is %.2f MB",
				workerIP, availableMemoryMB, requiredMemoryMB,
			))
		}

		if requiredCPUMHz > 0 && availableCPUMHz <= 0 {
			resourceErrors = append(resourceErrors, fmt.Sprintf(
				"worker_ip %s must specify cpu in workers.json (allocated jobs require %.2f MHZ)",
				workerIP, requiredCPUMHz,
			))
		} else if requiredCPUMHz > 0 && availableCPUMHz < requiredCPUMHz {
			resourceErrors = append(resourceErrors, fmt.Sprintf(
				"worker_ip %s, available cpu is %.2f MHZ, required cpu is %.2f MHZ",
				workerIP, availableCPUMHz, requiredCPUMHz,
			))
		}
	}
	if err := rows.Err(); err != nil {
		return bucket.DatabaseError(err)
	}

	if len(resourceErrors) != 0 {
		return fmt.Errorf("%w\n%s", bucket.ErrInsufficientResource, strings.Join(resourceErrors, "\n"))
	}

	return nil
}

func ValidateMinAllocationsCount(tx *sql.Tx, jobWorkspace *workspace.DefaultWorkspace) error {
	jobNames, err := jobWorkspace.GetJobs()
	if err != nil {
		return err
	}

	var validationErrors []string
	for _, jobName := range jobNames {
		manifest, err := jobWorkspace.GetJobManifest(jobName)
		if err != nil {
			return err
		}

		minCount := manifest.MinRequiredAllocations()
		if minCount == 0 {
			continue
		}

		count, err := data.CountJobAllocations(tx, jobName, false)
		if err != nil {
			return err
		}
		if count < minCount {
			validationErrors = append(validationErrors, fmt.Sprintf(
				"job %s has %d allocation(s), min_allocations_count is %d",
				jobName, count, minCount,
			))
		}
	}

	if len(validationErrors) != 0 {
		return fmt.Errorf("%w\n%s", bucket.ErrInsufficientAllocations, strings.Join(validationErrors, "\n"))
	}

	return nil
}
