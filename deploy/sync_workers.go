// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"maand/bucket"
	"maand/worker"
)

func syncWorkerFiles(rt *bucket.Runtime, bucketID, workerIP string) error {
	if err := runWorkerCommand(
		rt,
		workerIP,
		[]string{fmt.Sprintf("mkdir -p /opt/worker/%s", bucketID)},
		nil,
	); err != nil {
		return err
	}
	return runRsync(rt, bucketID, workerIP)
}

func syncWorkers(rt *bucket.Runtime, bucketID string, workers []string, jobs []string, applyRules bool) error {
	mergeFiles := make([]string, 0, len(workers))
	defer func() {
		for _, file := range mergeFiles {
			_ = os.Remove(file)
		}
	}()

	if err := worker.EnsureSSHStateDir(); err != nil {
		return err
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
		sem  = make(chan struct{}, len(workers))
	)

	for _, workerIP := range workers {
		mergePath, err := writeRsyncFilter(workerIP, jobs, applyRules)
		if err != nil {
			return err
		}
		mergeFiles = append(mergeFiles, mergePath)

		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := syncWorkerFiles(rt, bucketID, ip); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("worker %s: %w", ip, err))
				mu.Unlock()
			}
		}(workerIP)
	}

	wg.Wait()
	return joinErrors("rsync failed", errs)
}

func buildRsyncFilterLines(jobs []string, applyRules bool) []string {
	lines := make([]string, 0, len(jobs)+3)
	if applyRules {
		for _, job := range jobs {
			lines = append(lines, fmt.Sprintf("+ jobs/%s/\n", job))
		}
		lines = append(lines, "- jobs/*\n")
	}
	lines = append(lines, "- jobs/**/*.tpl\n")
	return lines
}

func writeRsyncFilter(workerIP string, jobs []string, applyRules bool) (string, error) {
	lines := buildRsyncFilterLines(jobs, applyRules)

	mergePath := path.Join(bucket.TempLocation, "workers", workerIP+".rsync")
	if err := os.MkdirAll(path.Dir(mergePath), 0o755); err != nil {
		return "", bucket.UnexpectedError(err)
	}
	if err := os.WriteFile(mergePath, []byte(strings.Join(lines, "")), 0o644); err != nil {
		return "", err
	}
	return mergePath, nil
}
