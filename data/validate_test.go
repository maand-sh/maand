// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"maand/bucket"
	"maand/worker"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerSyncError(t *testing.T) {
	err := &WorkerSyncError{WorkerIP: "10.0.0.1", Reason: "update_seq mismatch"}
	assert.Contains(t, err.Error(), "10.0.0.1")
	assert.Contains(t, err.Error(), "maand deploy")

	for code, reason := range map[int]string{
		1: "bucket id mismatch",
		2: "worker id mismatch",
		3: "update_seq mismatch",
	} {
		cmd := exec.Command("sh", "-c", fmt.Sprintf("exit %d", code))
		runErr := cmd.Run()
		require.Error(t, runErr)
		syncErr := workerSyncError("10.0.0.2", runErr)
		var typed *WorkerSyncError
		require.ErrorAs(t, syncErr, &typed)
		assert.Equal(t, reason, typed.Reason)
	}

	syncErr := workerSyncError("10.0.0.3", assert.AnError)
	var plain *WorkerSyncError
	assert.False(t, errors.As(syncErr, &plain))
}

func TestValidateBucketUpdateSeq(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	worker.SetTestHooks(&worker.TestHooks{
		ExecuteCommand: func(_ *bucket.Runtime, _ string, _ []string, _ []string) error {
			return nil
		},
	})
	t.Cleanup(worker.ClearTestHooks)

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	require.NoError(t, ValidateBucketUpdateSeq(tx, &bucket.Runtime{}, []string{"10.0.0.1"}))
}

func TestValidateBucketUpdateSeqWorkerFailure(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	cmd := exec.Command("sh", "-c", "exit 2")
	runErr := cmd.Run()
	require.Error(t, runErr)

	worker.SetTestHooks(&worker.TestHooks{
		ExecuteCommand: func(_ *bucket.Runtime, _ string, _ []string, _ []string) error {
			return runErr
		},
	})
	t.Cleanup(worker.ClearTestHooks)

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = ValidateBucketUpdateSeq(tx, &bucket.Runtime{}, []string{"10.0.0.1"})
	require.Error(t, err)
	var syncErr *WorkerSyncError
	require.ErrorAs(t, err, &syncErr)
	assert.Equal(t, "worker id mismatch", syncErr.Reason)
}
