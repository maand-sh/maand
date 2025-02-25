// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package run_command

import (
	"errors"
	"fmt"
	"strings"
)

type RunCommandError struct {
	Errs map[string]error
}

func (e *RunCommandError) Error() string {
	var message []string
	for workerIP, err := range e.Errs {
		if errors.Unwrap(err).Error() == "exit status 255" {
			message = append(message, fmt.Sprintf("worker: %s, timed out", workerIP))
		}
	}
	return fmt.Sprintf("Command failed %s", strings.Join(message, "; "))
}
