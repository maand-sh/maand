// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package healthcheck

import (
	"database/sql"
	"fmt"
	"time"

	"maand/bucket"
	"maand/data"
)

// CheckWorkers verifies SSH port reachability on every worker in the catalog.
func CheckWorkers(tx *sql.Tx, wait bool) error {
	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}
	if len(workers) == 0 {
		fmt.Println("worker health check skipped: no workers")
		return nil
	}

	sshPort, err := bucket.SSHPort()
	if err != nil {
		return err
	}

	run := func() error {
		for _, workerIP := range workers {
			if err := probeTCP(workerIP, sshPort, defaultProbeTimeout); err != nil {
				return fmt.Errorf("worker %s ssh port %d: %w", workerIP, sshPort, err)
			}
		}
		return nil
	}

	if wait {
		var lastErr error
		for attempt := 1; attempt <= waitRetryAttempts; attempt++ {
			lastErr = run()
			if lastErr == nil {
				fmt.Println("worker health check passed")
				return nil
			}
			fmt.Printf("worker health check failed (attempt %d/%d), retrying...\n", attempt, waitRetryAttempts)
			time.Sleep(waitRetryInterval)
		}
		return &WorkerHealthCheckError{Err: lastErr}
	}

	if err := run(); err != nil {
		fmt.Println("worker health check failed")
		return &WorkerHealthCheckError{Err: err}
	}
	fmt.Println("worker health check passed")
	return nil
}
