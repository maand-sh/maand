// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"

	"maand/data"
)

// buildWorkerJobsList returns jobs.json entries for a worker, excluding removed allocations.
func buildWorkerJobsList(tx *sql.Tx, workerIP string) ([]WorkerJobs, error) {
	allocatedJobs, err := data.GetAllocatedJobs(tx, workerIP)
	if err != nil {
		return nil, err
	}

	workerJobs := make([]WorkerJobs, 0, len(allocatedJobs))
	for _, job := range allocatedJobs {
		removed, err := data.IsAllocationRemoved(tx, workerIP, job)
		if err != nil {
			return nil, err
		}
		if removed == 1 {
			continue
		}
		disabled, err := data.IsAllocationDisabled(tx, workerIP, job)
		if err != nil {
			return nil, err
		}
		workerJobs = append(workerJobs, WorkerJobs{Job: job, Disabled: disabled})
	}
	return workerJobs, nil
}
