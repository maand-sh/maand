// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"errors"
	"fmt"
	"os/exec"
	"sync"

	"maand/bucket"
	"maand/worker"
)

// WorkerSyncError reports that a worker's worker.json does not match maand.db.
type WorkerSyncError struct {
	WorkerIP string
	Reason   string
}

func (e *WorkerSyncError) Error() string {
	return fmt.Sprintf("worker %s: %s (run maand deploy)", e.WorkerIP, e.Reason)
}

// ValidateBucketUpdateSeq verifies each worker's worker.json matches bucket_id,
// worker_id, and update_seq in maand.db before manual job control.
func ValidateBucketUpdateSeq(tx *sql.Tx, rt *bucket.Runtime, workers []string) error {
	bucketID, err := GetBucketID(tx)
	if err != nil {
		return err
	}

	updateSeq, err := GetBucketUpdateSeq(tx)
	if err != nil {
		return err
	}

	var (
		errs = make(map[string]error)
		mu   sync.Mutex
		wg   sync.WaitGroup
	)

	for _, workerIP := range workers {
		workerID, err := GetWorkerID(tx, workerIP)
		if err != nil {
			return err
		}

		wg.Add(1)
		go func(tWorkerID, tWorkerIP string) {
			defer wg.Done()
			cmd := fmt.Sprintf(
				"python3 /opt/worker/%s/bin/worker.py %s %s %d",
				bucketID, bucketID, tWorkerID, updateSeq,
			)
			err := worker.ExecuteCommand(rt, tWorkerIP, []string{cmd}, nil)
			if err != nil {
				mu.Lock()
				errs[tWorkerIP] = err
				mu.Unlock()
			}
		}(workerID, workerIP)
	}
	wg.Wait()

	var syncErrors []error
	for workerIP, err := range errs {
		if syncErr := workerSyncError(workerIP, err); syncErr != nil {
			syncErrors = append(syncErrors, syncErr)
		}
	}
	return errors.Join(syncErrors...)
}

func workerSyncError(workerIP string, err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case 1:
			return &WorkerSyncError{WorkerIP: workerIP, Reason: "bucket id mismatch"}
		case 2:
			return &WorkerSyncError{WorkerIP: workerIP, Reason: "worker id mismatch"}
		case 3:
			return &WorkerSyncError{WorkerIP: workerIP, Reason: "update_seq mismatch"}
		}
	}
	return fmt.Errorf("worker %s: %w", workerIP, err)
}
