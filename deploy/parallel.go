// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import "sync"

// runParallelWorkers runs fn for each workerIP with at most parallelism concurrent calls.
// fn must not use a shared *sql.Tx; database access from callbacks must be synchronized externally.
func runParallelWorkers(workerIPs []string, parallelism int, fn func(workerIP string) error) error {
	if len(workerIPs) == 0 {
		return nil
	}
	if parallelism < 1 {
		parallelism = 1
	}
	if parallelism > len(workerIPs) {
		parallelism = len(workerIPs)
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
		sem  = make(chan struct{}, parallelism)
	)

	for _, workerIP := range workerIPs {
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := fn(ip); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(workerIP)
	}

	wg.Wait()
	return joinErrors("", errs)
}

// runWorkerBatches processes workers in contiguous batches of batchSize (parallel within each batch).
func runWorkerBatches(workerIPs []string, batchSize int, fn func(workerIP string) error) error {
	if batchSize < 1 {
		batchSize = 1
	}
	for i := 0; i < len(workerIPs); i += batchSize {
		end := i + batchSize
		if end > len(workerIPs) {
			end = len(workerIPs)
		}
		if err := runParallelWorkers(workerIPs[i:end], end-i, fn); err != nil {
			return err
		}
	}
	return nil
}
