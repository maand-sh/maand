// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package worker

import (
	"fmt"

	"maand/bucket"
)

// RemoteCommandError reports a failed SSH command to a worker from the bucket container.
type RemoteCommandError struct {
	WorkerIP string
	Err      error
}

func (e *RemoteCommandError) Error() string {
	return fmt.Sprintf("worker %s: %v", e.WorkerIP, e.Err)
}

func (e *RemoteCommandError) Unwrap() error {
	return e.Err
}

func (e *RemoteCommandError) Is(target error) bool {
	return target == bucket.ErrRunCommand
}

func remoteError(workerIP string, err error) error {
	if err == nil {
		return nil
	}
	return &RemoteCommandError{WorkerIP: workerIP, Err: err}
}
