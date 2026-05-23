// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"

	"maand/bucket"
	"maand/data"
	"maand/healthcheck"
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

	return healthcheck.HealthCheck(tx, rt, true, job, true)
}

func handleUpdatedAllocations(tx *sql.Tx, rt *bucket.Runtime, bucketID, job string) error {
	updatedAllocations, err := data.GetUpdatedAllocations(tx, job)
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

	restartCmd := runnerCommand(bucketID, "restart", job)
	if err := runWorkerBatches(active, parallelism, func(workerIP string) error {
		env, err := allocationVersionEnv(tx, job, workerIP)
		if err != nil {
			return err
		}
		return runWorkerCommand(rt, workerIP, []string{restartCmd}, env)
	}); err != nil {
		return fmt.Errorf("restart updated allocations: %w", err)
	}

	return healthcheck.HealthCheck(tx, rt, true, job, true)
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
		stopCmd := runnerCommand(bucketID, "stop", alloc.Job)
		if assumeDead {
			runWorkerCommandOrAssumeDead(rt, alloc.WorkerIP, []string{stopCmd}, nil)
		} else if err := runWorkerCommand(rt, alloc.WorkerIP, []string{stopCmd}, nil); err != nil {
			return fmt.Errorf("worker %s job %s: %w", alloc.WorkerIP, alloc.Job, err)
		}
		if alloc.Removed || alloc.Disabled {
			if err := removeJobDirectoryFromWorker(rt, bucketID, alloc.WorkerIP, alloc.Job, assumeDead); err != nil {
				return err
			}
		}
	}

	return nil
}

// reconcileRemovedAndDisabledAllocations stops removed/disabled allocations and removes
// their job trees from workers. Workers dropped from workers.json also lose the entire
// /opt/worker/<bucket_id>/ tree once all of their removed allocations are processed.
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
		if activeCount > 1 {
			if err := healthcheck.HealthCheck(tx, rt, true, job, true); err != nil {
				return err
			}
		}
	}

	for workerIP := range offCatalogWorkers {
		if err := removeWorkerBucketFromWorker(rt, bucketID, workerIP); err != nil {
			return err
		}
	}

	return nil
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
