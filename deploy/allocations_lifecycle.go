// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"

	"maand/bucket"
	"maand/data"
)

func handleUpdatedAllocationsWithPolicy(tx *sql.Tx, rt *bucket.Runtime, bucketID, job string, opts Options) error {
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

	byAction := map[string][]string{
		rolloutActionRestart: nil,
		rolloutActionReload:  nil,
	}

	for _, workerIP := range updatedAllocations {
		hashChanged, versionOnly, err := allocationRolloutReason(tx, job, workerIP, opts.Force)
		if err != nil {
			return err
		}
		action, err := resolveWorkerLifecycleAction(tx, job, workerIP, opts, policy, globs, hashChanged, versionOnly)
		if err != nil {
			return err
		}
		if action == rolloutActionSync {
			continue
		}
		byAction[action] = append(byAction[action], workerIP)
	}

	for _, action := range []string{rolloutActionRestart, rolloutActionReload} {
		workers := byAction[action]
		if len(workers) == 0 {
			continue
		}
		lifecycleCmd := runnerCommand(bucketID, action, job)
		lifecycleFn := func(workerIP string) error {
			env, err := allocationVersionEnv(tx, job, workerIP)
			if err != nil {
				return err
			}
			return runWorkerCommand(rt, workerIP, runnerCmdCtx(job, "rollout", action, bucketID), []string{lifecycleCmd}, env)
		}
		if err := rolloutRestartBatches(tx, rt, bucketID, job, workers, lifecycleFn); err != nil {
			return err
		}
	}
	return nil
}

func allocationRolloutReason(tx *sql.Tx, job, workerIP string, force bool) (hashChanged, versionOnly bool, err error) {
	namespace := fmt.Sprintf("%s_allocation", job)
	allocID, err := data.GetAllocationID(tx, workerIP, job)
	if err != nil {
		return false, false, err
	}
	current, previous, ok, err := data.GetAllocationHash(tx, namespace, allocID)
	if err != nil {
		return false, false, err
	}
	if ok && previous != "" && current != previous {
		return true, false, nil
	}
	needsVersion, err := data.AllocationNeedsVersionRollout(tx, job, workerIP)
	if err != nil {
		return false, false, err
	}
	if needsVersion {
		return false, true, nil
	}
	if force {
		return false, false, nil
	}
	return false, false, nil
}

func resolveWorkerLifecycleAction(
	tx *sql.Tx,
	job, workerIP string,
	opts Options,
	policy string,
	globs []string,
	hashChanged, versionOnly bool,
) (string, error) {
	action, _, err := resolveAllocationLifecycle(tx, job, workerIP, opts, policy, globs, hashChanged, versionOnly)
	return action, err
}
