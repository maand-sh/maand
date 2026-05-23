// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package healthcheck

import (
	"fmt"
	"strings"

	"maand/bucket"
)

const waitRetryAttempts = 30

// HealthCheckError reports a failed health check for one job.
type HealthCheckError struct {
	Job string
	Err error
}

func (e *HealthCheckError) Error() string {
	return fmt.Sprintf("job %s: %v", e.Job, e.Err)
}

func (e *HealthCheckError) Unwrap() error {
	return e.Err
}

func (e *HealthCheckError) Is(target error) bool {
	return target == bucket.ErrHealthCheckFailed
}

// BatchHealthCheckError reports failures across multiple jobs.
type BatchHealthCheckError struct {
	Failures []HealthCheckError
}

func (e *BatchHealthCheckError) Error() string {
	messages := make([]string, 0, len(e.Failures))
	for _, failure := range e.Failures {
		messages = append(messages, failure.Error())
	}
	return strings.Join(messages, "; ")
}

func (e *BatchHealthCheckError) Is(target error) bool {
	return target == bucket.ErrHealthCheckFailed
}

func (e *BatchHealthCheckError) Unwrap() error {
	if len(e.Failures) == 1 {
		return &e.Failures[0]
	}
	return bucket.ErrHealthCheckFailed
}

func newBatchHealthCheckError(failures []HealthCheckError) error {
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("%w", &BatchHealthCheckError{Failures: failures})
}
