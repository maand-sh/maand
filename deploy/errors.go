// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"fmt"
	"strings"
)

// JobError reports a failure while deploying a single job.
type JobError struct {
	Job string
	Err error
}

func (e *JobError) Error() string {
	return fmt.Sprintf("job %s: %v", e.Job, e.Err)
}

func (e *JobError) Unwrap() error {
	return e.Err
}

func joinErrors(prefix string, errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	messages := make([]string, len(errs))
	for i, err := range errs {
		messages[i] = err.Error()
	}
	if prefix == "" {
		return fmt.Errorf("%s", strings.Join(messages, "\n"))
	}
	return fmt.Errorf("%s:\n%s", prefix, strings.Join(messages, "\n"))
}
