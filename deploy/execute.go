// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package deploy pushes job artifacts to worker nodes via the bucket container.
//
// Pipeline per deployment sequence: reconcile removed/disabled allocations once,
// then per wave: stage workers → refresh plan hashes (skip detection) → per job:
// pre_deploy → stage → rsync → hash → start/restart allocations → post_deploy → promote.
// Before contacting workers, deploy verifies local tools (bash, ssh, rsync, python3,
// bun when needed) and each worker has required tools (python3, make, rsync, bash, timeout,
// and sudo/rsync when use_sudo is set).
// Job-command KV is checkpointed per job into the deploy transaction so it commits with
// partial deploys (SQLite allows only one writer; a second connection would lock).
//
// Partial deploy: the transaction commits even when some jobs fail. Jobs that finished
// promote are skipped on the next maand deploy (JobNeedsRollout); only jobs with new or
// updated allocation hashes are staged and rolled out again. Final rsync runs per
// successfully deployed job (only that job's tree).
//
// Jobs run sequentially against the database transaction; worker rsync runs in parallel
// without touching the transaction.
package deploy

import (
	"database/sql"
	_ "embed"
	"log"
	"os"

	"maand/bucket"
	"maand/certs"
	"maand/data"
	"maand/jobcommand"
	"maand/kv"
)

//go:embed runner.py
var runnerPy []byte

//go:embed worker.py
var workerPy []byte

// UpdateSeq increments the bucket update sequence in its own committed transaction.
func UpdateSeq(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	updateSeq, err := data.GetBucketUpdateSeq(tx)
	if err != nil {
		return err
	}
	if err := data.SetBucketUpdateSeq(tx, updateSeq+1); err != nil {
		return err
	}
	return tx.Commit()
}

// Execute deploys jobs to all workers, optionally filtered by job name.
// When opts.Force is true, jobs already promoted on all allocations are staged and
// restarted anyway. When opts.SyncOnly is true, files are rsynced and hashes promoted
// without lifecycle actions; new allocations are rejected.
func Execute(jobsFilter []string, opts Options) error {
	db, err := data.OpenDatabase(true)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	if err := os.RemoveAll(bucket.TempLocation); err != nil {
		return bucket.UnexpectedError(err)
	}
	defer func() {
		_ = os.RemoveAll(bucket.TempLocation)
	}()

	if err := UpdateSeq(db); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := kv.Initialize(tx); err != nil {
		return err
	}

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}

	maxDeploymentSequence, err := data.GetMaxDeploymentSeq(tx)
	if err != nil {
		return err
	}

	updateSeq, err := data.GetBucketUpdateSeq(tx)
	if err != nil {
		return err
	}

	rt, err := setupDeployRuntime(bucketID, bucket.NewRunContext("deploy", updateSeq))
	if err != nil {
		return err
	}
	cancel := jobcommand.StartRuntimeAPI(tx)
	defer func() {
		cancel()
		if rt != nil {
			_ = rt.Stop()
		}
	}()

	var (
		deployFailures []error
		deployedJobs   []string
	)
	jobsFilter = normalizeJobFilter(jobsFilter)

	if err := checkDeployPrerequisites(workers); err != nil {
		return err
	}

	if err := reconcileRemovedAndDisabledAllocations(tx, rt, bucketID); err != nil {
		return err
	}

	for deploymentSeq := 0; deploymentSeq <= maxDeploymentSequence; deploymentSeq++ {
		availableJobs, err := data.GetJobsByDeploymentSeq(tx, deploymentSeq)
		if err != nil {
			return err
		}
		jobs := selectJobsForDeploy(availableJobs, jobsFilter)
		if len(jobs) == 0 {
			continue
		}

		if err := prepareWorkersFiles(tx, workers); err != nil {
			return err
		}

		if err := refreshPlanHashesForJobs(tx, jobs); err != nil {
			return err
		}

		jobsToStage := make([]string, 0, len(jobs))
		for _, job := range jobs {
			if !opts.Force {
				needsRollout, err := JobNeedsRollout(tx, job)
				if err != nil {
					return err
				}
				if !needsRollout {
					log.Printf("deploy: skip job %q (already promoted on all allocations)", job)
					if rt != nil {
						_ = rt.LogEvent("", "deploy_skip", map[string]string{
							"job":    job,
							"reason": "already_promoted",
						})
					}
					continue
				}
			}

			preErr := executePreJobCommandsForJob(tx, rt, job)
			if persistErr := persistJobCommandKV(tx, job); persistErr != nil {
				deployFailures = append(deployFailures, persistErr)
			}
			if preErr != nil {
				deployFailures = append(deployFailures, preErr)
				continue
			}
			jobsToStage = append(jobsToStage, job)
		}

		if len(jobsToStage) == 0 {
			continue
		}

		for _, job := range jobsToStage {
			if err := prepareJobsFiles(tx, []string{job}); err != nil {
				return err
			}
			if err := syncWorkers(rt, bucketID, workers, []string{job}, true); err != nil {
				return err
			}
			if err := updateAllocationHash(tx, []string{job}); err != nil {
				return err
			}

			deployErr := deployJob(tx, rt, bucketID, job, opts)
			if persistErr := persistJobCommandKV(tx, job); persistErr != nil {
				deployFailures = append(deployFailures, persistErr)
			}
			if deployErr != nil {
				deployFailures = append(deployFailures, deployErr)
				continue
			}
			deployedJobs = append(deployedJobs, job)
		}
	}

	if len(deployedJobs) > 0 {
		if err := finalSyncDeployedJobs(tx, rt, bucketID, deployedJobs); err != nil {
			deployFailures = append(deployFailures, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}

	certs.PushMetrics(db)

	return joinErrors("deploy failed", deployFailures)
}
