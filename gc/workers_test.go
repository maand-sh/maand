// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gc

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"maand/bucket"
	"maand/worker"

	_ "github.com/mattn/go-sqlite3"
)

func TestPurgeRemovedAllocationWorkerData_liveWorker(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:gcworkerslive?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		CREATE TABLE worker (worker_id TEXT, worker_ip TEXT PRIMARY KEY, position INT);
		INSERT INTO worker (worker_id, worker_ip, position) VALUES ('w1', '10.0.0.1', 0);
	`)
	require.NoError(t, err)

	var commands []string
	worker.SetTestHooks(&worker.TestHooks{
		ExecuteCommand: func(_ *bucket.Runtime, workerIP string, _ bucket.CommandContext, cmds []string, _ []string) error {
			assert.Equal(t, "10.0.0.1", workerIP)
			commands = append(commands, cmds...)
			return nil
		},
	})
	t.Cleanup(worker.ClearTestHooks)

	tx, err := db.Begin()
	require.NoError(t, err)

	rt := &bucket.Runtime{}
	require.NoError(t, purgeRemovedAllocationWorkerData(rt, tx, "bucket-1", []removedAllocation{
		{Job: "vault", WorkerIP: "10.0.0.1"},
	}))
	require.NoError(t, tx.Commit())

	require.Len(t, commands, 1)
	assert.Contains(t, commands[0], `/opt/worker/bucket-1/jobs/vault`)
}

func TestPurgeRemovedAllocationWorkerData_deadWorker(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:gcworkersdead?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE TABLE worker (worker_id TEXT, worker_ip TEXT PRIMARY KEY, position INT)`)
	require.NoError(t, err)

	called := false
	worker.SetTestHooks(&worker.TestHooks{
		ExecuteCommand: func(_ *bucket.Runtime, workerIP string, _ bucket.CommandContext, _ []string, _ []string) error {
			called = true
			assert.Equal(t, "10.0.0.99", workerIP)
			return assert.AnError
		},
	})
	t.Cleanup(worker.ClearTestHooks)

	tx, err := db.Begin()
	require.NoError(t, err)

	rt := &bucket.Runtime{}
	require.NoError(t, purgeRemovedAllocationWorkerData(rt, tx, "bucket-1", []removedAllocation{
		{Job: "app", WorkerIP: "10.0.0.99"},
	}))
	require.NoError(t, tx.Commit())
	assert.True(t, called)
}
