// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"errors"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/jobcommand"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunErrorIsJobCommandFailed(t *testing.T) {
	runErr := &jobcommand.RunError{
		Job:     "app",
		Command: "command_deploy",
		Failures: []jobcommand.WorkerFailure{
			{WorkerIP: "10.0.0.1", Err: errors.New("exit 1")},
		},
	}

	assert.ErrorIs(t, runErr, bucket.ErrJobCommandFailed)
}

func TestNotFoundErrorMessage(t *testing.T) {
	err := &jobcommand.NotFoundError{Job: "app", Command: "missing", Event: "pre_deploy"}
	assert.Contains(t, err.Error(), "app")
	assert.Contains(t, err.Error(), "missing")
	assert.Contains(t, err.Error(), "pre_deploy")
}

func TestJobCommandRejectsUnregisteredCommand(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeMinimalJob(t, "app", `{"selectors":["worker"]}`)
	runBuild(t)

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() {
		_ = tx.Rollback()
	}()

	err = jobcommand.JobCommand(tx, nil, "app", "not_registered", "cli", 1, false, nil)
	require.Error(t, err)
	var notFound *jobcommand.NotFoundError
	assert.ErrorAs(t, err, &notFound)
}
