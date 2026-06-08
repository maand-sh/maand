// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcontrol

import (
	"database/sql"
	"fmt"

	"maand/bucket"
	"maand/data"
	"maand/healthcheck"
	"maand/jobcommand"
)

const jobControlEvent = "job_control"

// runJobControlCommands executes manifest job_control commands when registered; otherwise false.
func runJobControlCommands(
	tx *sql.Tx,
	rt *bucket.Runtime,
	job string,
	target Target,
	healthCheck bool,
	workerFilter []string,
) (handled bool, err error) {
	commands, err := data.GetJobCommands(tx, job, jobControlEvent)
	if err != nil {
		return false, &JobRunError{Job: job, Target: target, Err: err}
	}
	if len(commands) == 0 {
		return false, nil
	}

	allocatedWorkers, err := data.GetActiveAllocations(tx, job)
	if err != nil {
		return false, &JobRunError{Job: job, Target: target, Err: err}
	}

	selected := filterWorkers(allocatedWorkers, workerFilter)
	if len(selected) == 0 {
		return true, nil
	}

	extraEnv := []string{fmt.Sprintf("TARGET=%s", target)}
	for _, command := range commands {
		if err := jobcommand.JobCommand(
			tx,
			rt,
			job,
			command,
			jobControlEvent,
			len(selected),
			true,
			extraEnv,
		); err != nil {
			return true, &JobRunError{Job: job, Target: target, Err: err}
		}
	}

	if healthCheck {
		if _, err := healthcheck.HealthCheck(tx, rt, true, false, job, true); err != nil {
			return true, &JobRunError{Job: job, Target: target, Err: err}
		}
	}

	return true, nil
}
