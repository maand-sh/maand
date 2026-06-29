// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocationRolloutQueries(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	_, err = tx.Exec(`
		UPDATE hash SET previous_hash = NULL WHERE namespace = 'api_allocation' AND key = 'alloc-1';
		UPDATE hash SET current_hash = 'staged', previous_hash = 'live'
		WHERE namespace = 'api_allocation' AND key = 'alloc-2';
	`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	newAllocs, err := GetNewAllocations(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1"}, newAllocs)

	updated, err := GetUpdatedAllocations(tx, "api")
	require.NoError(t, err)
	assert.Empty(t, updated)

	updatedNonRemoved, err := GetUpdatedNonRemovedAllocations(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.2"}, updatedNonRemoved)

	current, previous, ok, err := GetAllocationHash(tx, "api_allocation", "alloc-1")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "hash-a", current)
	assert.Equal(t, "", previous)

	_, _, ok, err = GetAllocationHash(tx, "api_allocation", "missing")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestJobCommandAndHealthQueries(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	_, err = tx.Exec(`
		INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
		VALUES ('job-api', 'api', 'init', 'pre_deploy', '', '', '');
		UPDATE job SET health_check = '{"checks":[{"type":"tcp","port":"http_port"}]}' WHERE name = 'api';
	`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	commands, err := GetJobCommands(tx, "api", "pre_deploy")
	require.NoError(t, err)
	assert.Equal(t, []string{"init"}, commands)

	jobs, err := GetJobsWithCommand(tx, "init", "pre_deploy")
	require.NoError(t, err)
	assert.Equal(t, []string{"api"}, jobs)

	parallel, err := GetMaxConcurrentUpgrades(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, 1, parallel)

	health, err := GetJobHealthCheck(tx, "api")
	require.NoError(t, err)
	require.NotNil(t, health)
	assert.Len(t, health.Checks, 1)

	port, err := GetJobPortNumber(tx, "api", "http_port")
	require.NoError(t, err)
	assert.Equal(t, 8080, port)
}

func TestListStoppedAllocations(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	stopped, err := ListStoppedAllocations(tx)
	require.NoError(t, err)
	require.Len(t, stopped, 1)
	assert.Equal(t, "10.0.0.2", stopped[0].WorkerIP)
	assert.True(t, stopped[0].Disabled)
	assert.False(t, stopped[0].Removed)
}

func TestInsertBucketRecord(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`DELETE FROM bucket`)
	require.NoError(t, err)
	require.NoError(t, InsertBucketRecord(tx, "new-bucket"))
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	initialized, err := BucketInitialized(tx)
	require.NoError(t, err)
	assert.True(t, initialized)

	bucketID, err := GetBucketID(tx)
	require.NoError(t, err)
	assert.Equal(t, "new-bucket", bucketID)
}

func TestUpdateHashInsertsMissingRow(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, UpdateHash(tx, "api_allocation", "new-alloc", "hash-x"))
	require.NoError(t, tx.Commit())

	var count int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM hash WHERE namespace = 'api_allocation' AND key = 'new-alloc'`,
	).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestGetUpdatedAllocationsFindsActiveWorkers(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	_, err = tx.Exec(`
		UPDATE hash SET current_hash = 'staged', previous_hash = 'live'
		WHERE namespace = 'api_allocation' AND key = 'alloc-1';
	`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	updated, err := GetUpdatedAllocations(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1"}, updated)
}

func TestMigrateToV1SkipsExistingColumns(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, migrateToV1(tx))
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, migrateToV1(tx))
	require.NoError(t, tx.Rollback())
}
