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

func effectiveBatchSize(requested, total int) int {
	if requested < 1 {
		return total
	}
	if requested > total {
		return total
	}
	return requested
}

func rolloutStartBatches(
	tx *sql.Tx,
	rt *bucket.Runtime,
	bucketID, job string,
	candidates []string,
	startFn func(workerIP string) error,
) error {
	if len(candidates) == 0 {
		return nil
	}

	active, err := activeWorkers(candidates, tx, job)
	if err != nil {
		return err
	}
	if len(active) == 0 {
		return nil
	}

	resolved, err := ResolveDeployOrder(tx, job, active)
	if err != nil {
		return err
	}

	parallelism, err := data.GetDeployParallelCount(tx, job)
	if err != nil {
		return err
	}
	batchSize := effectiveBatchSize(parallelism, len(resolved.Ordered))
	totalBatches := batchCount(len(resolved.Ordered), batchSize)

	for i := 0; i < len(resolved.Ordered); i += batchSize {
		end := i + batchSize
		if end > len(resolved.Ordered) {
			end = len(resolved.Ordered)
		}
		batch := resolved.Ordered[i:end]
		if err := runParallelWorkers(batch, len(batch), startFn); err != nil {
			return err
		}
		ctx := BatchContext{
			Job:             job,
			Phase:           deployPhaseNew,
			BatchIndex:      i / batchSize,
			BatchCount:      totalBatches,
			BatchAllocation: append([]string(nil), batch...),
			DeployOrder:     resolved.FullOrder,
			OrderSource:     resolved.Source,
		}
		if err := executeAfterAllocationStarted(tx, rt, job, batch, ctx); err != nil {
			return err
		}
	}

	_, err = healthcheck.HealthCheck(tx, rt, true, false, job, true)
	return err
}

func rolloutRestartBatches(
	tx *sql.Tx,
	rt *bucket.Runtime,
	bucketID, job string,
	candidates []string,
	restartFn func(workerIP string) error,
) error {
	if len(candidates) == 0 {
		return nil
	}

	active, err := activeWorkers(candidates, tx, job)
	if err != nil {
		return err
	}
	if len(active) == 0 {
		return nil
	}

	resolved, err := ResolveDeployOrder(tx, job, active)
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
	totalBatches := batchCount(len(resolved.Ordered), parallelism)

	for i := 0; i < len(resolved.Ordered); i += parallelism {
		end := i + parallelism
		if end > len(resolved.Ordered) {
			end = len(resolved.Ordered)
		}
		batch := resolved.Ordered[i:end]
		if err := runParallelWorkers(batch, len(batch), restartFn); err != nil {
			return fmt.Errorf("restart updated allocations: %w", err)
		}
		ctx := BatchContext{
			Job:             job,
			Phase:           deployPhaseUpdate,
			BatchIndex:      i / parallelism,
			BatchCount:      totalBatches,
			BatchAllocation: append([]string(nil), batch...),
			DeployOrder:     resolved.FullOrder,
			OrderSource:     resolved.Source,
		}
		if err := executeAfterAllocationStarted(tx, rt, job, batch, ctx); err != nil {
			return err
		}
		if _, err := healthcheck.HealthCheck(tx, rt, true, false, job, true); err != nil {
			return err
		}
	}

	return nil
}

// JobNeedsRollout reports whether the job still needs a deploy wave: new workers,
// staged content or versions not yet promoted, or missing hash rows on non-removed
// allocations (including disabled). Reconcile stop for newly disabled allocations
// runs at the start of deploy even when this returns false.
func JobNeedsRollout(tx *sql.Tx, job string) (bool, error) {
	workers, err := data.GetNonRemovedAllocations(tx, job)
	if err != nil {
		return false, err
	}
	namespace := fmt.Sprintf("%s_allocation", job)
	for _, workerIP := range workers {
		allocID, err := data.GetAllocationID(tx, workerIP, job)
		if err != nil {
			return false, err
		}
		var hashCount int
		err = tx.QueryRow(
			`SELECT count(*) FROM hash WHERE namespace = ? AND key = ?`,
			namespace, allocID,
		).Scan(&hashCount)
		if err != nil {
			return false, bucket.DatabaseError(err)
		}
		if hashCount == 0 {
			return true, nil
		}
	}

	newAllocations, err := data.GetNewAllocations(tx, job)
	if err != nil {
		return false, err
	}
	if len(newAllocations) > 0 {
		return true, nil
	}
	updatedAllocations, err := data.GetUpdatedNonRemovedAllocations(tx, job)
	if err != nil {
		return false, err
	}
	if len(updatedAllocations) > 0 {
		return true, nil
	}

	versionPending, err := data.GetVersionPendingNonRemovedAllocations(tx, job)
	if err != nil {
		return false, err
	}
	return len(versionPending) > 0, nil
}
