// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"

	"maand/bucket"
	"maand/data"
)

// finalSyncDeployedJobs rsyncs each successfully deployed job only to workers with an
// active allocation for that job. Worker metadata (worker.json, jobs.json) is refreshed
// once per worker before the first job sync for that worker.
func finalSyncDeployedJobs(
	tx *sql.Tx,
	rt *bucket.Runtime,
	bucketID string,
	deployedJobs []string,
) error {
	refreshedWorkers := make(map[string]bool, len(deployedJobs))

	for _, job := range deployedJobs {
		workers, err := data.GetActiveAllocations(tx, job)
		if err != nil {
			return err
		}
		if len(workers) == 0 {
			continue
		}

		for _, workerIP := range workers {
			if refreshedWorkers[workerIP] {
				continue
			}
			if err := prepareOneWorkerFiles(tx, workerIP); err != nil {
				return err
			}
			refreshedWorkers[workerIP] = true
		}

		if err := syncWorkers(rt, bucketID, workers, []string{job}, true); err != nil {
			return err
		}
	}
	return nil
}
