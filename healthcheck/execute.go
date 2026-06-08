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
	defaultJobParallelism = 4
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
func HealthCheck(tx *sql.Tx, rt *bucket.Runtime, wait bool, job string, verbose bool) error {
	spec, err := data.GetJobHealthCheck(tx, job)
	if err != nil {
		return err
	}

	commands, err := data.GetJobCommands(tx, job, "health_check")
	if err != nil {
		return err
	}

	hasProbes := spec != nil && len(spec.Checks) > 0
	hasCommands := len(commands) > 0
	if !hasProbes && !hasCommands {
		fmt.Printf("health check skipped: %s (no health_check config or commands)\n", job)
		return nil
	}

	attempts, interval := waitConfig(spec)

	runChecks := func() error {
		if hasProbes {
			if err := runManifestHealthChecks(tx, job, spec); err != nil {
				return err
			}
		}
		for _, commandName := range commands {
			if err := jobcommand.JobCommand(tx, rt, job, commandName, "health_check", 1, verbose, nil); err != nil {
				return err
			}
		}
		return nil
	}

	if wait {
		var lastErr error
		for attempt := 1; attempt <= attempts; attempt++ {
			lastErr = runChecks()
			if lastErr == nil {
				fmt.Printf("health check passed: %s\n", job)
				return nil
			}
			fmt.Printf("health check failed: %s (attempt %d/%d), retrying...\n", job, attempt, attempts)
			time.Sleep(interval)
		}
		return &HealthCheckError{Job: job, Err: lastErr}
	}

	if err := runChecks(); err != nil {
		fmt.Printf("health check failed: %s\n", job)
		return &HealthCheckError{Job: job, Err: err}
	}

	fmt.Printf("health check passed: %s\n", job)
	return nil
}

// RunJobs health-checks multiple jobs using the same transaction and runtime.
func RunJobs(tx *sql.Tx, rt *bucket.Runtime, wait, verbose bool, jobNames []string) error {
	if err := CheckWorkers(tx, wait); err != nil {
		return err
	}

	jobNames = utils.Unique(jobNames)
	if len(jobNames) == 0 {
		return nil
	}

	failures := make([]HealthCheckError, 0)
	var failureMu sync.Mutex
	var waitGroup sync.WaitGroup
	semaphore := make(chan struct{}, defaultJobParallelism)

	for _, jobName := range jobNames {
		waitGroup.Add(1)
		semaphore <- struct{}{}

		go func(name string) {
			defer waitGroup.Done()
			defer func() { <-semaphore }()

			if err := HealthCheck(tx, rt, wait, name, verbose); err != nil {
				failureMu.Lock()
				if hcErr, ok := err.(*HealthCheckError); ok {
					failures = append(failures, *hcErr)
				} else {
					failures = append(failures, HealthCheckError{Job: name, Err: err})
				}
				failureMu.Unlock()
			}
		}(jobName)
	}

	waitGroup.Wait()
	return newBatchHealthCheckError(failures)
}

// Execute runs health checks for jobs in the bucket (optionally filtered by name).
func Execute(wait, verbose bool, jobsComma string) error {
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
	defer func() {
		_ = tx.Rollback()
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

	rt, err := bucket.SetupRuntime(bucketID)
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

	if err := RunJobs(tx, rt, wait, verbose, jobNames); err != nil {
		return err
	}
	return tx.Commit()
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
