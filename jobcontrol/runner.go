// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcontrol

import (
	"database/sql"
	"fmt"
	"sync"

	"maand/bucket"
	"maand/data"
	"maand/healthcheck"
	"maand/worker"
)

func runnerCommand(bucketID string, target Target, job string) string {
	return fmt.Sprintf(
		"python3 /opt/worker/%s/bin/runner.py %s %s --jobs %s",
		bucketID, bucketID, target, job,
	)
}

// runRunnerTarget runs the default runner.py target on active allocations in parallel batches.
func runRunnerTarget(
	tx *sql.Tx,
	rt *bucket.Runtime,
	bucketID, job string,
	target Target,
	healthCheck bool,
	workerFilter []string,
) error {
	allocatedWorkers, err := data.GetActiveAllocations(tx, job)
	if err != nil {
		return &JobRunError{Job: job, Target: target, Err: err}
	}

	selected := filterWorkers(allocatedWorkers, workerFilter)
	if len(selected) == 0 {
		return nil
	}

	parallelBatchCount, err := data.GetUpdateParallelCount(tx, job)
	if err != nil {
		return &JobRunError{Job: job, Target: target, Err: err}
	}
	if parallelBatchCount < 1 {
		parallelBatchCount = 1
	}

	command := runnerCommand(bucketID, target, job)
	workerCount := len(selected)

	for i := 0; i < workerCount; i += parallelBatchCount {
		end := i + parallelBatchCount
		if end > workerCount {
			end = workerCount
		}
		batch := selected[i:end]

		if err := runWorkerBatch(rt, job, target, batch, command); err != nil {
			return &JobRunError{Job: job, Target: target, Failures: err}
		}

		if healthCheck {
			if err := healthcheck.HealthCheck(tx, rt, true, job, true); err != nil {
				return &JobRunError{Job: job, Target: target, Err: err}
			}
		}
	}

	return nil
}

func filterWorkers(workers, workerFilter []string) []string {
	if len(workerFilter) == 0 {
		out := make([]string, len(workers))
		copy(out, workers)
		return out
	}
	selected := make([]string, 0, len(workers))
	for _, workerIP := range workers {
		if workerSelected(workerIP, workerFilter) {
			selected = append(selected, workerIP)
		}
	}
	return selected
}

func runWorkerBatch(rt *bucket.Runtime, job string, target Target, workerIPs []string, command string) []WorkerFailure {
	var (
		wg       sync.WaitGroup
		failures []WorkerFailure
		mu       sync.Mutex
	)

	cmdCtx := bucket.CommandContext{
		Job:    job,
		Phase:  "job_control",
		Action: string(target),
		Cmd:    command,
	}

	for _, workerIP := range workerIPs {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			if err := worker.ExecuteCommand(rt, ip, cmdCtx, []string{command}, nil); err != nil {
				mu.Lock()
				failures = append(failures, WorkerFailure{WorkerIP: ip, Err: err})
				mu.Unlock()
			}
		}(workerIP)
	}

	wg.Wait()
	return failures
}
