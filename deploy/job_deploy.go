// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/healthcheck"
	"maand/jobcommand"
)

// deployJob runs the full deploy pipeline for one job on the current deployment sequence.
// Called sequentially per job because *sql.Tx is not safe for concurrent use (SQLite).
func deployJob(tx *sql.Tx, rt *bucket.Runtime, bucketID, job string, force bool) error {
	commands, err := data.GetJobCommands(tx, job, "job_control")
	if err != nil {
		return &JobError{Job: job, Err: err}
	}

	if len(commands) > 0 {
		return deployJobWithCommands(tx, rt, job, commands, force)
	}

	if err := handleNewAllocations(tx, rt, bucketID, job); err != nil {
		return &JobError{Job: job, Err: fmt.Errorf("start new allocations: %w", err)}
	}
	if err := handleUpdatedAllocations(tx, rt, bucketID, job, force); err != nil {
		return &JobError{Job: job, Err: fmt.Errorf("restart updated allocations: %w", err)}
	}

	return finalizeJobDeploy(tx, rt, job)
}

func deployJobWithCommands(tx *sql.Tx, rt *bucket.Runtime, job string, commands []string, force bool) error {
	if !force {
		needsRollout, err := JobNeedsRollout(tx, job)
		if err != nil {
			return &JobError{Job: job, Err: err}
		}
		if !needsRollout {
			return nil
		}
	}

	allocations, err := data.GetAllocatedWorkers(tx, job)
	if err != nil {
		return &JobError{Job: job, Err: err}
	}

	newAllocations, err := data.GetNewAllocations(tx, job)
	if err != nil {
		return &JobError{Job: job, Err: err}
	}
	updatedAllocations, err := allocationsNeedingRestart(tx, job, force)
	if err != nil {
		return &JobError{Job: job, Err: err}
	}

	extraEnv := []string{
		fmt.Sprintf("UPDATED_ALLOCATIONS=%s", strings.Join(updatedAllocations, ",")),
		fmt.Sprintf("NEW_ALLOCATIONS=%s", strings.Join(newAllocations, ",")),
	}

	for _, command := range commands {
		if err := jobcommand.JobCommand(
			tx, rt, job, command, "job_control", len(allocations), false, extraEnv,
		); err != nil {
			return &JobError{Job: job, Err: err}
		}
	}

	if _, err := healthcheck.HealthCheck(tx, rt, true, false, job, true); err != nil {
		return &JobError{Job: job, Err: err}
	}
	return finalizeJobDeploy(tx, rt, job)
}

// finalizeJobDeploy runs post_deploy hooks and promotes allocation hashes after rollout.
func finalizeJobDeploy(tx *sql.Tx, rt *bucket.Runtime, job string) error {
	if err := executePostJobCommands(tx, rt, job); err != nil {
		return err
	}
	return promoteAllocationHash(tx, job)
}
