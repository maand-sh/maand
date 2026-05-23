// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package worker

import (
	"fmt"
	"strings"
	"sync"

	"maand/bucket"
	"maand/prereq"
)

// PrerequisitesError reports missing tools on a worker before deploy.
type PrerequisitesError struct {
	WorkerIP string
	Missing  []string
}

func (e *PrerequisitesError) Error() string {
	if len(e.Missing) == 0 {
		return fmt.Sprintf("worker %s: prerequisite check failed", e.WorkerIP)
	}
	return fmt.Sprintf("worker %s missing prerequisites: %s", e.WorkerIP, strings.Join(e.Missing, ", "))
}

func (e *PrerequisitesError) Is(target error) bool {
	return target == bucket.ErrWorkerPrerequisites
}

// CheckPrerequisites verifies deploy prerequisites on each worker over SSH.
func CheckPrerequisites(workerIPs []string, useSudo bool) error {
	spec := prereq.DeployWorkerSpec
	spec.UseSudo = useSudo
	return checkPrerequisites(workerIPs, spec)
}

// CheckRunCommandPrerequisites verifies SSH/bash prerequisites for run_command on workers.
func CheckRunCommandPrerequisites(workerIPs []string, useSudo bool) error {
	spec := prereq.RunCommandWorkerSpec
	spec.UseSudo = useSudo
	return checkPrerequisites(workerIPs, spec)
}

func checkPrerequisites(workerIPs []string, spec prereq.WorkerSpec) error {
	if len(workerIPs) == 0 {
		return nil
	}

	if err := EnsureSSHStateDir(); err != nil {
		return err
	}

	script := prereq.BuildWorkerCheckScript(spec)
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for _, workerIP := range workerIPs {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			if err := checkWorkerPrerequisites(ip, script); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(workerIP)
	}

	wg.Wait()
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w:\n%s", bucket.ErrWorkerPrerequisites, joinPrerequisiteErrors(errs))
}

func checkWorkerPrerequisites(workerIP, script string) error {
	output, err := RunRemoteScriptCombined(workerIP, strings.NewReader(script))
	if err == nil {
		return nil
	}

	missing := prereq.ParseMissingPrerequisites(output)
	if len(missing) > 0 {
		return &PrerequisitesError{WorkerIP: workerIP, Missing: missing}
	}
	return fmt.Errorf("worker %s: %w", workerIP, err)
}

func joinPrerequisiteErrors(errs []error) string {
	messages := make([]string, len(errs))
	for i, err := range errs {
		messages[i] = err.Error()
	}
	return strings.Join(messages, "\n")
}
