// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"
	"log"

	"maand/bucket"
	"maand/data"
	"maand/healthcheck"
	"maand/utils"
)

func activeWorkers(workers []string, tx *sql.Tx, job string) ([]string, error) {
	active := make([]string, 0, len(workers))
	for _, workerIP := range workers {
		ok, err := data.IsAllocationActive(tx, workerIP, job)
		if err != nil {
			return nil, err
		}
		if ok {
			active = append(active, workerIP)
		}
	}
	return active, nil
}

func handleNewAllocations(tx *sql.Tx, rt *bucket.Runtime, bucketID, job string) error {
	newAllocations, err := data.GetNewAllocations(tx, job)
	if err != nil {
		return err
	}
	if len(newAllocations) == 0 {
		return nil
	}

	active, err := activeWorkers(newAllocations, tx, job)
	if err != nil {
		return err
	}

	startCmd := runnerCommand(bucketID, "start", job)
	if err := runParallelWorkers(active, len(active), func(workerIP string) error {
		env, err := allocationVersionEnv(tx, job, workerIP)
		if err != nil {
			return err
		}
		return runWorkerCommand(rt, workerIP, []string{startCmd}, env)
	}); err != nil {
		return fmt.Errorf("start new allocations: %w", err)
	}

	_, err = healthcheck.HealthCheck(tx, rt, true, false, job, true)
	return err
}

func allocationsNeedingRestart(tx *sql.Tx, job string, force bool) ([]string, error) {
	// New allocations (no promoted hash yet) are started by handleNewAllocations and must
	// never be restarted by the update path. GetVersionPendingAllocations would otherwise
	// include them on a fresh deploy (current_version empty vs build target), causing a
	// redundant second rollout right after the initial start.
	newAllocations, err := data.GetNewAllocations(tx, job)
	if err != nil {
		return nil, err
	}

	if !force {
		updated, err := data.GetUpdatedAllocations(tx, job)
		if err != nil {
			return nil, err
		}
		versionPending, err := data.GetVersionPendingAllocations(tx, job)
		if err != nil {
			return nil, err
		}
		candidates := utils.Unique(append(updated, versionPending...))
		return utils.Difference(candidates, newAllocations), nil
	}
	active, err := data.GetActiveAllocations(tx, job)
	if err != nil {
		return nil, err
	}
	return utils.Difference(active, newAllocations), nil
}

func handleUpdatedAllocations(tx *sql.Tx, rt *bucket.Runtime, bucketID, job string, force bool) error {
	updatedAllocations, err := allocationsNeedingRestart(tx, job, force)
	if err != nil {
		return err
	}
	if len(updatedAllocations) == 0 {
		return nil
	}

	active, err := activeWorkers(updatedAllocations, tx, job)
	if err != nil {
		return err
	}

	parallelism, err := data.GetUpdateParallelCount(tx, job)
	if err != nil {
		return err
	}
	if parallelism < 1 {
		parallelism = 1
	}

	restartCmd := runnerCommand(bucketID, "restart", job)
	restart := func(workerIP string) error {
		env, err := allocationVersionEnv(tx, job, workerIP)
		if err != nil {
			return err
		}
		return runWorkerCommand(rt, workerIP, []string{restartCmd}, env)
	}

	// Rolling upgrade: restart in batches of update_parallel_count and run the job
	// health check after each batch so a bad rollout is caught before the next wave.
	for i := 0; i < len(active); i += parallelism {
		end := i + parallelism
		if end > len(active) {
			end = len(active)
		}
		batch := active[i:end]
		if err := runParallelWorkers(batch, len(batch), restart); err != nil {
			return fmt.Errorf("restart updated allocations: %w", err)
		}
		if _, err := healthcheck.HealthCheck(tx, rt, true, false, job, true); err != nil {
			return err
		}
	}

	return nil
}

func handleStoppedAllocations(tx *sql.Tx, rt *bucket.Runtime, bucketID string, jobs []string) error {
	jobFilter := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		jobFilter[job] = struct{}{}
	}

	stopped, err := data.ListStoppedAllocations(tx)
	if err != nil {
		return err
	}

	for _, alloc := range stopped {
		if _, ok := jobFilter[alloc.Job]; !ok {
			continue
		}
		if err := reconcileStoppedAllocation(tx, rt, bucketID, alloc, false); err != nil {
			return err
		}
	}
	return nil
}

func reconcileStoppedAllocation(
	tx *sql.Tx,
	rt *bucket.Runtime,
	bucketID string,
	alloc data.StoppedAllocation,
	assumeDead bool,
) error {
	namespace := fmt.Sprintf("%s_allocation", alloc.Job)
	allocID, err := data.GetAllocationID(tx, alloc.WorkerIP, alloc.Job)
	if err != nil {
		return err
	}

	previousHash, err := data.GetPreviousHash(tx, namespace, allocID)
	if err != nil {
		return err
	}

	if previousHash != "" {
		if alloc.Disabled {
			log.Printf("deploy: stop disabled allocation %s on %s", alloc.Job, alloc.WorkerIP)
		}
		stopCmd := runnerCommand(bucketID, "stop", alloc.Job)
		if assumeDead {
			runWorkerCommandOrAssumeDead(rt, alloc.WorkerIP, []string{stopCmd}, nil)
		} else if err := runWorkerCommand(rt, alloc.WorkerIP, []string{stopCmd}, nil); err != nil {
			return fmt.Errorf("worker %s job %s: %w", alloc.WorkerIP, alloc.Job, err)
		}
		if alloc.Removed {
			if err := removeJobDeployArtifactsFromWorker(rt, bucketID, alloc.WorkerIP, alloc.Job, assumeDead); err != nil {
				return err
			}
		}
	}

	if alloc.Removed {
		if err := data.RemoveAllocationHash(tx, alloc.Job, allocID); err != nil {
			return err
		}
	}

	return nil
}

// reconcileRemovedAndDisabledAllocations stops removed/disabled allocations. Removed
// allocations also lose deployed job files (data/ and logs/ preserved until maand gc).
// Disabled allocations are stopped only; deploy artifacts and hash state are kept for
// re-enable. maand gc deletes the full jobs/<job>/ tree when purging removed rows.
// Workers dropped
// from workers.json also lose the entire /opt/worker/<bucket_id>/ tree once all of their
// removed allocations are processed.
func reconcileRemovedAndDisabledAllocations(
	tx *sql.Tx,
	rt *bucket.Runtime,
	bucketID string,
) error {
	currentWorkers, err := data.LoadWorkerCatalog(tx)
	if err != nil {
		return err
	}

	stopped, err := data.ListStoppedAllocations(tx)
	if err != nil {
		return err
	}

	jobsNeedingHealthCheck := make(map[string]struct{})
	offCatalogWorkers := make(map[string]struct{})

	for _, alloc := range stopped {
		wasRunning, err := allocationWasDeployed(tx, alloc.WorkerIP, alloc.Job)
		if err != nil {
			return err
		}

		assumeDead := data.StoppedAllocationAssumeDead(alloc, currentWorkers)

		if err := reconcileStoppedAllocation(tx, rt, bucketID, alloc, assumeDead); err != nil {
			return err
		}

		if wasRunning {
			jobsNeedingHealthCheck[alloc.Job] = struct{}{}
		}

		if alloc.Removed && !currentWorkers.Contains(alloc.WorkerIP) && wasRunning {
			offCatalogWorkers[alloc.WorkerIP] = struct{}{}
		}
	}

	for job := range jobsNeedingHealthCheck {
		activeCount, err := countActiveAllocations(tx, job)
		if err != nil {
			return err
		}
		// Health-check the job once at the end whenever any active allocation
		// remains. Skip only when the job has no active allocations left (fully
		// removed/disabled), since there is nothing to probe.
		if activeCount > 0 {
			if _, err := healthcheck.HealthCheck(tx, rt, true, false, job, true); err != nil {
				return err
			}
		}
	}

	for workerIP := range offCatalogWorkers {
		if err := removeWorkerBucketFromWorker(rt, bucketID, workerIP); err != nil {
			return err
		}
	}

	return purgeJobCommandKVForInactiveJobs(tx, stopped)
}

func allocationWasDeployed(tx *sql.Tx, workerIP, job string) (bool, error) {
	namespace := fmt.Sprintf("%s_allocation", job)
	allocID, err := data.GetAllocationID(tx, workerIP, job)
	if err != nil {
		return false, err
	}
	previousHash, err := data.GetPreviousHash(tx, namespace, allocID)
	if err != nil {
		return false, err
	}
	return previousHash != "", nil
}

func countActiveAllocations(tx *sql.Tx, job string) (int, error) {
	workers, err := data.GetActiveAllocations(tx, job)
	if err != nil {
		return 0, err
	}
	return len(workers), nil
}
