// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package jobcontrol runs start/stop/restart/status across job allocations,
// using registered job_control commands when present or runner.py otherwise.
package jobcontrol

import (
	"database/sql"
	"sync"

	"maand/bucket"
	"maand/data"
	"maand/healthcheck"
)

// Request configures a job control run.
type Request struct {
	JobsCSV     string
	WorkersCSV  string
	Target      Target
	HealthCheck bool
}

// Execute runs job control for optional job and worker filters.
// jobsCSV and workersCSV are comma-separated; empty means all allocated jobs/workers.
func Execute(jobsCSV, workersCSV, target string, healthCheck bool) error {
	parsedTarget, err := ParseTarget(target)
	if err != nil {
		return err
	}
	return ExecuteRequest(Request{
		JobsCSV:     jobsCSV,
		WorkersCSV:  workersCSV,
		Target:      parsedTarget,
		HealthCheck: healthCheck,
	})
}

// ExecuteRequest is the structured entry point for job control.
func ExecuteRequest(req Request) error {
	db, err := data.OpenDatabase(true)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	return runControl(tx, req)
}

func runControl(tx *sql.Tx, req Request) error {
	filters := ParseFilters(req.JobsCSV, req.WorkersCSV)

	allJobs, err := data.GetAllAllocatedJobs(tx)
	if err != nil {
		return err
	}
	allWorkers, err := data.GetAllWorkers(tx)
	if err != nil {
		return err
	}
	if err := filters.validateAgainst(allJobs, allWorkers); err != nil {
		return err
	}

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

	updateSeq, err := data.GetBucketUpdateSeq(tx)
	if err != nil {
		return err
	}

	rt, err := bucket.SetupRuntime(bucketID, bucket.NewRunContext("job", updateSeq))
	if err != nil {
		return err
	}
	defer func() {
		_ = rt.Stop()
	}()

	cancel, err := healthcheck.PrepareRuntime(tx)
	if err != nil {
		return err
	}
	defer cancel()

	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}
	if err := data.ValidateBucketUpdateSeq(tx, rt, workers); err != nil {
		return err
	}

	maxDeploymentSequence, err := data.GetMaxDeploymentSeq(tx)
	if err != nil {
		return err
	}

	var jobFailures []JobRunError

	for deploymentSeq := 0; deploymentSeq <= maxDeploymentSequence; deploymentSeq++ {
		jobsAtSeq, err := data.GetJobsByDeploymentSeq(tx, deploymentSeq)
		if err != nil {
			return err
		}

		selectedJobs := selectJobs(jobsAtSeq, filters.Jobs)
		seqFailures := runJobsInParallel(tx, rt, bucketID, selectedJobs, req, filters.Workers)
		jobFailures = append(jobFailures, seqFailures...)
	}

	return newControlError(jobFailures)
}

func runJobsInParallel(
	tx *sql.Tx,
	rt *bucket.Runtime,
	bucketID string,
	jobs []string,
	req Request,
	workerFilter []string,
) []JobRunError {
	if len(jobs) == 0 {
		return nil
	}

	var (
		wg       sync.WaitGroup
		failures []JobRunError
		mu       sync.Mutex
	)

	for _, job := range jobs {
		wg.Add(1)
		go func(jobName string) {
			defer wg.Done()
			if err := controlJob(tx, rt, bucketID, jobName, req, workerFilter); err != nil {
				var jobErr *JobRunError
				if asJobRun(err, &jobErr) {
					mu.Lock()
					failures = append(failures, *jobErr)
					mu.Unlock()
					return
				}
				mu.Lock()
				failures = append(failures, JobRunError{Job: jobName, Target: req.Target, Err: err})
				mu.Unlock()
			}
		}(job)
	}

	wg.Wait()
	return failures
}

func controlJob(
	tx *sql.Tx,
	rt *bucket.Runtime,
	bucketID, job string,
	req Request,
	workerFilter []string,
) error {
	handled, err := runJobControlCommands(
		tx,
		rt,
		job,
		req.Target,
		req.HealthCheck,
		workerFilter,
	)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}
	return runRunnerTarget(
		tx,
		rt,
		bucketID,
		job,
		req.Target,
		req.HealthCheck,
		workerFilter,
	)
}

func asJobRun(err error, target **JobRunError) bool {
	if err == nil {
		return false
	}
	if jobErr, ok := err.(*JobRunError); ok {
		*target = jobErr
		return true
	}
	return false
}
