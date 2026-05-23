// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package runcommand

import (
	"errors"
	"fmt"
	"strings"

	"maand/bucket"
)

// RunCommandError reports one or more worker command failures.
type RunCommandError struct {
	Failures []WorkerCommandFailure
}

// WorkerCommandFailure ties a worker IP to the error returned for that worker.
type WorkerCommandFailure struct {
	WorkerIP string
	Err      error
}

func (e *RunCommandError) Error() string {
	messages := make([]string, 0, len(e.Failures))
	for _, failure := range e.Failures {
		messages = append(messages, formatWorkerFailure(failure))
	}
	return strings.Join(messages, "; ")
}

func (e *RunCommandError) Is(target error) bool {
	return target == bucket.ErrRunCommand
}

func formatWorkerFailure(failure WorkerCommandFailure) string {
	if failure.Err == nil {
		return fmt.Sprintf("worker %s: unknown error", failure.WorkerIP)
	}

	errMsg := failure.Err.Error()
	switch {
	case strings.Contains(errMsg, "exit status 255"):
		return fmt.Sprintf("worker %s: timed out", failure.WorkerIP)
	case strings.Contains(errMsg, "exit status 1"):
		return fmt.Sprintf("worker %s: command failed (exit status 1)", failure.WorkerIP)
	case strings.Contains(errMsg, "command execution failed"):
		return fmt.Sprintf("worker %s: command execution failed", failure.WorkerIP)
	default:
		return fmt.Sprintf("worker %s: %v", failure.WorkerIP, failure.Err)
	}
}

func newRunCommandError(failures map[string]error) error {
	if len(failures) == 0 {
		return nil
	}

	failureList := make([]WorkerCommandFailure, 0, len(failures))
	for workerIP, err := range failures {
		failureList = append(failureList, WorkerCommandFailure{
			WorkerIP: workerIP,
			Err:      err,
		})
	}

	return fmt.Errorf("%w", &RunCommandError{Failures: failureList})
}

func errConcurrencyTooLow() error {
	return errors.New("batch size (-c) must be at least 1")
}

func errCommandRequired() error {
	return errors.New("command argument or workspace/command.sh is required")
}

func errEmptyCommand() error {
	return errors.New("command is empty")
}

func errWorkersAndLabelsTogether() error {
	return errors.New("specify either --workers or --labels, not both")
}

func errUnknownWorkers(workerIPs []string) error {
	return fmt.Errorf("workers not in this bucket: %v", workerIPs)
}

func errNoTargetWorkers() error {
	return errors.New("no workers matched the selection")
}
