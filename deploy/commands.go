// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"

	"maand/bucket"
	"maand/data"
	"maand/jobcommand"
)

func executePreJobCommands(tx *sql.Tx, rt *bucket.Runtime, jobs []string) error {
	for _, job := range jobs {
		if err := executePreJobCommandsForJob(tx, rt, job); err != nil {
			return err
		}
	}
	return nil
}

func executePreJobCommandsForJob(tx *sql.Tx, rt *bucket.Runtime, job string) error {
	commands, err := data.GetJobCommands(tx, job, "pre_deploy")
	if err != nil {
		return &JobError{Job: job, Err: err}
	}
	for _, command := range commands {
		if err := jobcommand.JobCommand(tx, rt, job, command, "pre_deploy", 1, true, nil); err != nil {
			return &JobError{Job: job, Err: err}
		}
	}
	return nil
}

func executePostJobCommands(tx *sql.Tx, rt *bucket.Runtime, job string) error {
	commands, err := data.GetJobCommands(tx, job, "post_deploy")
	if err != nil {
		return &JobError{Job: job, Err: err}
	}
	for _, command := range commands {
		if err := jobcommand.JobCommand(tx, rt, job, command, "post_deploy", 1, true, nil); err != nil {
			return &JobError{Job: job, Err: err}
		}
	}
	return nil
}
