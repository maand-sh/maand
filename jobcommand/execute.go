// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package jobcommand runs Python or Bun.js job commands inside the maand container across worker allocations.
package jobcommand

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"maand/bucket"
	"maand/data"
	"maand/kv"
	"maand/utils"
)

//go:embed maand.py
var MaandPy []byte

//go:embed maand.ts
var MaandTS []byte

// JobCommand runs commandName for jobName on all active allocations for event.
func JobCommand(
	tx *sql.Tx,
	rt *bucket.Runtime,
	jobName, commandName, event string,
	concurrency int,
	verbose bool,
	extraEnv []string,
) error {
	workerIPs, err := data.GetActiveAllocations(tx, jobName)
	if err != nil {
		return err
	}
	return JobCommandOnWorkers(tx, rt, jobName, commandName, event, workerIPs, concurrency, verbose, extraEnv)
}

// JobCommandOnWorkers runs commandName on the given worker IPs for event.
func JobCommandOnWorkers(
	tx *sql.Tx,
	rt *bucket.Runtime,
	jobName, commandName, event string,
	workerIPs []string,
	concurrency int,
	verbose bool,
	extraEnv []string,
) error {
	allowedCommands, err := data.GetJobCommands(tx, jobName, event)
	if err != nil {
		return err
	}
	if !commandAllowed(allowedCommands, commandName) {
		return &NotFoundError{Job: jobName, Command: commandName, Event: event}
	}

	if len(workerIPs) == 0 {
		return nil
	}
	workerIPs = utils.Unique(workerIPs)

	if err := validateHostRuntime(jobName, commandName); err != nil {
		return err
	}

	if err := prepareWorkerWorkspaces(tx, jobName, workerIPs, event, commandName); err != nil {
		return err
	}

	if concurrency < 1 {
		concurrency = 1
	}

	return runCommandOnWorkers(tx, rt, jobName, commandName, event, workerIPs, concurrency, verbose, extraEnv)
}

func runCommandOnWorkers(
	tx *sql.Tx,
	rt *bucket.Runtime,
	jobName, commandName, event string,
	workerIPs []string,
	concurrency int,
	verbose bool,
	extraEnv []string,
) error {
	type allocation struct {
		id       string
		workerIP string
		disabled int
	}

	allocations := make([]allocation, 0, len(workerIPs))
	for _, workerIP := range workerIPs {
		disabled, err := data.IsAllocationDisabled(tx, workerIP, jobName)
		if err != nil {
			return err
		}
		allocID, err := data.GetAllocationID(tx, workerIP, jobName)
		if err != nil {
			return err
		}
		allocations = append(allocations, allocation{
			id:       allocID,
			workerIP: workerIP,
			disabled: disabled,
		})
	}

	var (
		waitGroup sync.WaitGroup
		failures  []WorkerFailure
		failureMu sync.Mutex
		semaphore = make(chan struct{}, concurrency)
	)

	for _, alloc := range allocations {
		waitGroup.Add(1)
		semaphore <- struct{}{}

		go func(alloc allocation) {
			defer waitGroup.Done()
			defer func() { <-semaphore }()

			allocationIndex, err := allocationIndexForJobCommand(tx, jobName, alloc.workerIP)
			if err != nil {
				failureMu.Lock()
				failures = append(failures, WorkerFailure{WorkerIP: alloc.workerIP, Err: err})
				failureMu.Unlock()
				return
			}
			versionEnv, err := allocationVersionEnvForJobCommand(tx, jobName, alloc.workerIP)
			if err != nil {
				failureMu.Lock()
				failures = append(failures, WorkerFailure{WorkerIP: alloc.workerIP, Err: err})
				failureMu.Unlock()
				return
			}
			workerEnv := append(append([]string(nil), extraEnv...), versionEnv...)

			err = runCommandOnWorker(
				rt,
				alloc.id,
				jobName,
				alloc.workerIP,
				allocationIndex,
				alloc.disabled,
				commandName,
				event,
				verbose,
				workerEnv,
			)
			if err != nil {
				failureMu.Lock()
				failures = append(failures, WorkerFailure{WorkerIP: alloc.workerIP, Err: err})
				failureMu.Unlock()
			}
		}(alloc)
	}

	waitGroup.Wait()
	return newRunError(jobName, commandName, failures)
}

// Execute is the CLI entry point: open DB, start the job-command HTTP server, and run the command.
// When jobName is empty, commandName runs on every job that registers it for event.
func Execute(commandName, jobName, event string, concurrency int, verbose bool, extraEnv []string) error {
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

	if err := kv.Initialize(tx); err != nil {
		return err
	}

	cancelServer := StartRuntimeAPI(tx)
	defer cancelServer()

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

	rt, err := bucket.SetupRuntime(bucketID, bucket.NewRunContext("jobcommand", 0))
	if err != nil {
		return err
	}
	defer func() {
		_ = rt.Stop()
	}()

	if err := os.MkdirAll(bucket.TempLocation, 0o755); err != nil {
		return err
	}

	jobs, err := resolveJobsForCommand(tx, jobName, commandName, event)
	if err != nil {
		return err
	}

	var runErrors []error
	for _, job := range jobs {
		if len(jobs) > 1 {
			log.Printf("jobcommand: %s", job)
		}
		if err := JobCommand(tx, rt, job, commandName, event, concurrency, verbose, extraEnv); err != nil {
			runErrors = append(runErrors, fmt.Errorf("job %s: %w", job, err))
		}
	}

	if err := kv.PersistToSessionTransaction(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}

	return errors.Join(runErrors...)
}

func resolveJobsForCommand(tx *sql.Tx, jobName, commandName, event string) ([]string, error) {
	if jobName != "" {
		allowed, err := data.GetJobCommands(tx, jobName, event)
		if err != nil {
			return nil, err
		}
		if !commandAllowed(allowed, commandName) {
			return nil, &NotFoundError{Job: jobName, Command: commandName, Event: event}
		}
		return []string{jobName}, nil
	}

	jobs, err := data.GetJobsWithCommand(tx, commandName, event)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, &NotFoundError{Command: commandName, Event: event}
	}
	return jobs, nil
}
