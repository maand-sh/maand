// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"os"

	"maand/bucket"
	"maand/data"
)

// refreshPlanHashesForJobPlan stages one job and updates plan hashes. When the job
// registers pre_deploy hooks, they run first so rendered content matches real deploy
// staging (deploy also runs pre_deploy before rsync/restart).
func refreshPlanHashesForJobPlan(tx *sql.Tx, rt *bucket.Runtime, job string) error {
	commands, err := data.GetJobCommands(tx, job, "pre_deploy")
	if err != nil {
		return &JobError{Job: job, Err: err}
	}
	if len(commands) > 0 {
		if err := executePreJobCommandsForJob(tx, rt, job); err != nil {
			return err
		}
		if err := persistJobCommandKV(tx, job); err != nil {
			return err
		}
	}
	return refreshPlanHashesForJobs(tx, []string{job})
}

// refreshPlanHashesForJobs stages jobs under tmp/workers/ and updates <job>_allocation
// current_hash from the rendered tree. Run at deploy time (before JobNeedsRollout) so
// content changes since the last promote are visible without build knowing about hashes.
func refreshPlanHashesForJobs(tx *sql.Tx, jobs []string) error {
	if len(jobs) == 0 {
		return nil
	}

	if err := os.MkdirAll(bucket.TempLocation, 0o755); err != nil {
		return bucket.UnexpectedError(err)
	}

	if err := prepareJobsFiles(tx, jobs); err != nil {
		return err
	}
	return updateAllocationHash(tx, jobs)
}
