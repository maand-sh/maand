// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package healthcheck

import (
	"database/sql"
	"errors"

	"maand/bucket"
	"maand/data"
	"maand/jobcommand"
)

func markHealthCheckFailures(tx *sql.Tx, job string, err error) (bool, error) {
	var runErr *jobcommand.RunError
	if !errors.As(err, &runErr) {
		return false, nil
	}

	marked := false
	for _, failure := range runErr.Failures {
		if err := data.MarkAllocationHealthFailed(tx, job, failure.WorkerIP); err != nil {
			return marked, err
		}
		marked = true
	}
	return marked, nil
}

func runHealthCheckCommand(
	tx *sql.Tx,
	rt *bucket.Runtime,
	job, commandName string,
	verbose, updateHash bool,
) (bool, error) {
	cmdErr := jobcommand.JobCommand(tx, rt, job, commandName, "health_check", 10, verbose, nil)
	if cmdErr == nil {
		return false, nil
	}
	if !updateHash {
		return false, cmdErr
	}

	marked, err := markHealthCheckFailures(tx, job, cmdErr)
	if err != nil {
		return marked, err
	}
	return marked, cmdErr
}
