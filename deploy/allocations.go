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

func handleNewAllocations(tx *sql.Tx, rt *bucket.Runtime, bucketID, job string) error {
	newAllocations, err := data.GetNewAllocations(tx, job)
	if err != nil {
		return err
	}
	if len(newAllocations) == 0 {
		return nil
	}

	startCmd := runnerCommand(bucketID, "start", job)
	startFn := func(workerIP string) error {
		env, err := allocationVersionEnv(tx, job, workerIP)
		if err != nil {
			return err
		}
		return runWorkerCommand(rt, workerIP, runnerCmdCtx(job, "rollout", "start", bucketID), []string{startCmd}, env)
	}

	if err := rolloutStartBatches(tx, rt, bucketID, job, newAllocations, startFn); err != nil {
		return fmt.Errorf("start new allocations: %w", err)
	}
	return nil
}

func handleUpdatedAllocations(tx *sql.Tx, rt *bucket.Runtime, bucketID, job string, opts Options) error {
	updatedAllocations, err := allocationsNeedingRestart(tx, job, opts.Force)
	if err != nil {
		return err
	}
	if len(updatedAllocations) == 0 {
		return nil
	}

	policy, err := data.GetRestartPolicy(tx, job)
	if err != nil {
		return err
	}
	globs, err := data.GetRestartGlobs(tx, job)
	if err != nil {
		return err
	}

	restartWorkers := make([]string, 0)
	reloadWorkers := make([]string, 0)
	namespace := fmt.Sprintf("%s_allocation", job)

	for _, workerIP := range updatedAllocations {
		allocID, err := data.GetAllocationID(tx, workerIP, job)
		if err != nil {
			return err
		}

		current, previous, ok, err := data.GetAllocationHash(tx, namespace, allocID)
		if err != nil {
			return err
		}
		hashChanged := ok && previous != "" && previous != current

		needsVersion, err := data.AllocationNeedsVersionRollout(tx, job, workerIP)
		if err != nil {
			return err
		}
		versionOnly := needsVersion && !hashChanged

		manifests, err := data.GetAllocationFileManifests(tx, namespace, allocID)
		if err != nil {
			return err
		}
		legacyNoManifest := !manifests.HasPreviousFiles

		action, _ := resolveUpdateAction(
			opts.SyncOnly, policy, globs,
			manifests.Previous, manifests.Current,
			hashChanged, versionOnly, legacyNoManifest,
		)
		if action == rolloutActionSync {
			continue
		}
		if action == rolloutActionRestart {
			restartWorkers = append(restartWorkers, workerIP)
		} else {
			reloadWorkers = append(reloadWorkers, workerIP)
		}
	}

	if len(reloadWorkers) > 0 {
		reloadCmd := runnerCommand(bucketID, rolloutActionReload, job)
		reloadFn := func(workerIP string) error {
			env, err := allocationVersionEnv(tx, job, workerIP)
			if err != nil {
				return err
			}
			return runWorkerCommand(rt, workerIP, runnerCmdCtx(job, "rollout", rolloutActionReload, bucketID), []string{reloadCmd}, env)
		}
		if err := rolloutRestartBatches(tx, rt, bucketID, job, reloadWorkers, reloadFn); err != nil {
			return err
		}
	}

	if len(restartWorkers) > 0 {
		restartCmd := runnerCommand(bucketID, rolloutActionRestart, job)
		restartFn := func(workerIP string) error {
			env, err := allocationVersionEnv(tx, job, workerIP)
			if err != nil {
				return err
			}
			return runWorkerCommand(rt, workerIP, runnerCmdCtx(job, "rollout", rolloutActionRestart, bucketID), []string{restartCmd}, env)
		}
		if err := rolloutRestartBatches(tx, rt, bucketID, job, restartWorkers, restartFn); err != nil {
			return err
		}
	}

	return nil
}

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

func allocationsNeedingRestart(tx *sql.Tx, job string, force bool) ([]string, error) {
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
		stopCtx := runnerCmdCtx(alloc.Job, "reconcile", "stop", bucketID)
		if assumeDead {
			runWorkerCommandOrAssumeDead(rt, alloc.WorkerIP, stopCtx, []string{stopCmd}, nil)
		} else if err := runWorkerCommand(rt, alloc.WorkerIP, stopCtx, []string{stopCmd}, nil); err != nil {
			return fmt.Errorf("worker %s job %s: %w", alloc.WorkerIP, alloc.Job, err)
		}
		if err := executeAfterAllocationStopped(tx, rt, alloc.Job, alloc.WorkerIP); err != nil {
			return fmt.Errorf("worker %s job %s: %w", alloc.WorkerIP, alloc.Job, err)
		}
		if alloc.Removed {
			if err := removeJobDeployArtifactsFromWorker(rt, bucketID, alloc.WorkerIP, alloc.Job, assumeDead); err != nil {
				return err
			}
		}
	} else if alloc.Removed || alloc.Disabled {
		if rt != nil {
			_ = rt.LogEvent(alloc.WorkerIP, "reconcile_skip_stop", map[string]string{
				"job":      alloc.Job,
				"reason":   "not_deployed",
				"removed":  fmt.Sprintf("%t", alloc.Removed),
				"disabled": fmt.Sprintf("%t", alloc.Disabled),
			})
		}
	}

	if alloc.Removed {
		if err := data.RemoveAllocationHash(tx, alloc.Job, allocID); err != nil {
			return err
		}
	}

	return nil
}

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
		if activeCount > 0 {
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
