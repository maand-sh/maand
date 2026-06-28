// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package healthcheck runs built-in manifest probes and health_check job commands.
package healthcheck

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"maand/bucket"
	"maand/data"
	"maand/jobcommand"
	"maand/kv"
	"maand/utils"
)

const (
	defaultJobParallelism = 16
	waitRetryInterval     = time.Second
)

// PrepareRuntime loads the KV store and starts the job-command HTTP server on tx.
// Call the returned cancel function when finished.
func PrepareRuntime(tx *sql.Tx) (context.CancelFunc, error) {
	if err := kv.Initialize(tx); err != nil {
		return nil, err
	}
	return jobcommand.StartRuntimeAPI(tx), nil
}

// HealthCheck runs manifest probes and health_check commands for a job.
// When updateHash is true, failed health_check commands mark allocations for redeploy.
func HealthCheck(tx *sql.Tx, rt *bucket.Runtime, wait, updateHash bool, job string, verbose bool) (bool, error) {
	hasActive, err := data.JobHasActiveAllocations(tx, job)
	if err != nil {
		return false, err
	}
	if !hasActive {
		fmt.Printf("health check skipped: %s (no active allocations)\n", job)
		return false, nil
	}

	spec, err := data.GetJobHealthCheck(tx, job)
	if err != nil {
		return false, err
	}

	commands, err := data.GetJobCommands(tx, job, "health_check")
	if err != nil {
		return false, err
	}

	hasProbes := spec != nil && len(spec.Checks) > 0
	hasCommands := len(commands) > 0
	if !hasProbes && !hasCommands {
		fmt.Printf("health check skipped: %s (no health_check config or commands)\n", job)
		return false, nil
	}

	attempts, interval := waitConfig(spec)

	runChecks := func() (bool, error) {
		hashMarked := false
		if hasProbes {
			if err := runManifestHealthChecks(tx, job, spec); err != nil {
				return hashMarked, err
			}
		}
		for _, commandName := range commands {
			marked, err := runHealthCheckCommand(tx, rt, job, commandName, verbose, updateHash)
			hashMarked = hashMarked || marked
			if err != nil {
				return hashMarked, err
			}
		}
		return hashMarked, nil
	}

	if wait {
		var lastErr error
		var hashMarked bool
		for attempt := 1; attempt <= attempts; attempt++ {
			hashMarked, lastErr = runChecks()
			if lastErr == nil {
				fmt.Printf("health check passed: %s\n", job)
				return hashMarked, nil
			}
			fmt.Printf("health check failed: %s (attempt %d/%d), retrying...\n", job, attempt, attempts)
			time.Sleep(interval)
		}
		return hashMarked, &HealthCheckError{Job: job, Err: lastErr}
	}

	hashMarked, err := runChecks()
	if err != nil {
		fmt.Printf("health check failed: %s\n", job)
		return hashMarked, &HealthCheckError{Job: job, Err: err}
	}

	fmt.Printf("health check passed: %s\n", job)
	return hashMarked, nil
}

// RunJobs health-checks multiple jobs using the same transaction and runtime.
func RunJobs(tx *sql.Tx, rt *bucket.Runtime, wait, updateHash, verbose bool, jobNames []string) (bool, error) {
	if err := CheckWorkers(tx, wait); err != nil {
		return false, err
	}

	jobNames = utils.Unique(jobNames)
	if len(jobNames) == 0 {
		return false, nil
	}

	failures := make([]HealthCheckError, 0)
	hashMarked := false
	var failureMu sync.Mutex
	var waitGroup sync.WaitGroup
	semaphore := make(chan struct{}, defaultJobParallelism)

	for _, jobName := range jobNames {
		waitGroup.Add(1)
		semaphore <- struct{}{}

		go func(name string) {
			defer waitGroup.Done()
			defer func() { <-semaphore }()

			marked, err := HealthCheck(tx, rt, wait, updateHash, name, verbose)
			if marked || err != nil {
				failureMu.Lock()
				hashMarked = hashMarked || marked
				if err != nil {
					if hcErr, ok := err.(*HealthCheckError); ok {
						failures = append(failures, *hcErr)
					} else {
						failures = append(failures, HealthCheckError{Job: name, Err: err})
					}
				}
				failureMu.Unlock()
			}
		}(jobName)
	}

	waitGroup.Wait()
	return hashMarked, newBatchHealthCheckError(failures)
}

// Execute runs health checks for jobs in the bucket (optionally filtered by name).
func Execute(wait, verbose bool, jobsComma string, updateHash bool) error {
	db, err := data.OpenDatabase(true)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	cancel, err := PrepareRuntime(tx)
	if err != nil {
		return err
	}
	defer cancel()

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

	rt, err := bucket.SetupRuntime(bucketID, bucket.NewRunContext("healthcheck", 0))
	if err != nil {
		return err
	}
	defer func() {
		_ = rt.Stop()
	}()

	jobNames, err := data.GetJobs(tx)
	if err != nil {
		return err
	}

	jobFilter := parseJobFilter(jobsComma)
	if len(jobFilter) > 0 {
		unknown := utils.Difference(jobFilter, jobNames)
		if len(unknown) > 0 {
			return fmt.Errorf("jobs not in this bucket: %v", unknown)
		}
		jobNames = jobFilter
	}

	hashMarked, err := RunJobs(tx, rt, wait, updateHash, verbose, jobNames)
	if hashMarked {
		if err := tx.Commit(); err != nil {
			return bucket.DatabaseError(err)
		}
		committed = true
	}
	if err != nil {
		return err
	}
	if committed {
		return nil
	}
	if err := tx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}
	committed = true
	return nil
}

func parseJobFilter(jobsComma string) []string {
	if strings.TrimSpace(jobsComma) == "" {
		return nil
	}

	parts := strings.Split(jobsComma, ",")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			names = append(names, part)
		}
	}
	return names
}
