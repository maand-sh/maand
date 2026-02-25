package build

import (
	"database/sql"
	"fmt"
	"strings"

	"maand/bucket"
)

func Validate(tx *sql.Tx) error {
	rows, err := tx.Query(`
        SELECT
            a.worker_ip, w.available_memory_mb, w.available_cpu_mhz, sum(j.current_memory_mb) as needed_memory, sum(j.current_cpu_mhz) AS needed_cpu
        FROM
            allocations a JOIN job j ON j.name = a.job
                JOIN
            worker w ON w.worker_ip = a.worker_ip
        GROUP BY a.worker_ip
    `)
	if err != nil {
		return bucket.DatabaseError(err)
	}

	var errs []string
	for rows.Next() {
		var workerIP string
		var availableMemoryMB, availableCPUMHZ, neededMemoryMB, neededCPUMhz float64
		err = rows.Scan(&workerIP, &availableMemoryMB, &availableCPUMHZ, &neededMemoryMB, &neededCPUMhz)
		if err != nil {
			return err
		}

		// Only validate resources if the worker has a non-zero capacity defined.
		// A zero capacity is treated as "unspecified" (untracked) to avoid blocking
		// deployments on workers where resource limits are not explicitly configured.
		if availableMemoryMB > 0 && availableMemoryMB < neededMemoryMB {
			errs = append(errs, fmt.Sprintf("worker_ip %s, available memory is %.2f MB, required memory is %.2f MB", workerIP, availableMemoryMB, neededMemoryMB))
		}
		if availableCPUMHZ > 0 && availableCPUMHZ < neededCPUMhz {
			errs = append(errs, fmt.Sprintf("worker_ip %s, available cpu is %.2f MHZ, required cpu is %.2f MHZ", workerIP, availableCPUMHZ, neededCPUMhz))
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("%w\n%s", bucket.ErrInSufficientResource, strings.Join(errs, "\n"))
	}

	return nil
}
