// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerQueries(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	workers, err := GetWorkers(tx, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, workers)

	webWorkers, err := GetWorkers(tx, []string{"web"})
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, webWorkers)

	workerID, err := GetWorkerID(tx, "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "w1", workerID)

	labels, err := GetWorkerLabels(tx, "w1")
	require.NoError(t, err)
	assert.Equal(t, []string{"web"}, labels)

	allLabels, err := GetLabels(tx)
	require.NoError(t, err)
	assert.Equal(t, []string{"web"}, allLabels)

	tags, err := GetWorkerTags(tx, "w1")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"role": "primary"}, tags)

	mem, err := GetWorkerMemory(tx, "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "1024", mem)

	cpu, err := GetWorkerCPU(tx, "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "2000", cpu)

	position, err := GetWorkerPosition(tx, "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, 0, position)

	jobs, err := GetAllocatedJobs(tx, "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, []string{"api"}, jobs)

	activeJobs, err := GetActiveAllocatedJobs(tx, "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, []string{"api"}, activeJobs)

	allocatedWorkers, err := GetAllocatedWorkers(tx, "api")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2"}, allocatedWorkers)

	active, err := GetActiveAllocationsOrdered(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1"}, active)

	nonRemoved, err := GetNonRemovedAllocations(tx, "api")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2"}, nonRemoved)

	nonRemovedOrdered, err := GetNonRemovedAllocationsOrdered(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, nonRemovedOrdered)

	activeAllocs, err := GetActiveAllocations(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1"}, activeAllocs)

	hasActive, err := JobHasActiveAllocations(tx, "api")
	require.NoError(t, err)
	assert.True(t, hasActive)

	hasNonRemoved, err := JobHasNonRemovedAllocations(tx, "api")
	require.NoError(t, err)
	assert.True(t, hasNonRemoved)

	disabled, err := IsAllocationDisabled(tx, "10.0.0.2", "api")
	require.NoError(t, err)
	assert.Equal(t, 1, disabled)

	removed, err := IsAllocationRemoved(tx, "10.0.0.1", "api")
	require.NoError(t, err)
	assert.Equal(t, 0, removed)

	isActive, err := IsAllocationActive(tx, "10.0.0.1", "api")
	require.NoError(t, err)
	assert.True(t, isActive)

	allWorkers, err := GetAllWorkers(tx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2"}, allWorkers)

	allocatedIPs, err := GetAllocatedWorkerIPs(tx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2"}, allocatedIPs)

	catalog, err := LoadWorkerCatalog(tx)
	require.NoError(t, err)
	assert.True(t, catalog.Contains("10.0.0.1"))
	assert.False(t, catalog.Contains("10.0.0.99"))
}

func TestJobHasNoActiveAllocations(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 0);
		INSERT INTO job (job_id, name, version, update_parallel_count)
		VALUES ('job-empty', 'empty', '1.0.0', 1);
	`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	hasActive, err := JobHasActiveAllocations(tx, "empty")
	require.NoError(t, err)
	assert.False(t, hasActive)

	hasNonRemoved, err := JobHasNonRemovedAllocations(tx, "empty")
	require.NoError(t, err)
	assert.False(t, hasNonRemoved)
}
