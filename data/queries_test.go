// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobAndBucketQueries(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	jobs, err := GetJobs(tx)
	require.NoError(t, err)
	assert.Equal(t, []string{"api"}, jobs)

	allJobs, err := GetAllAllocatedJobs(tx)
	require.NoError(t, err)
	assert.Equal(t, []string{"api"}, allJobs)

	minMem, maxMem, err := GetJobMemoryLimits(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, "128", minMem)
	assert.Equal(t, "256", maxMem)

	mem, err := GetJobMemory(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, "128", mem)

	minCPU, maxCPU, err := GetJobCPULimits(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, "100", minCPU)
	assert.Equal(t, "200", maxCPU)

	cpu, err := GetJobCPU(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, "100", cpu)

	version, err := GetJobVersion(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", version)

	selectors, err := GetJobSelectors(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, []string{"web"}, selectors)

	allocID, err := GetAllocationID(tx, "10.0.0.1", "api")
	require.NoError(t, err)
	assert.Equal(t, "alloc-1", allocID)

	portMap, err := GetJobPortMap(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"http_port": "8080"}, portMap)

	assignments, err := GetAllJobPortAssignments(tx)
	require.NoError(t, err)
	assert.Equal(t, 8080, assignments["api"]["http_port"])

	initialized, err := BucketInitialized(tx)
	require.NoError(t, err)
	assert.True(t, initialized)

	bucketID, err := GetBucketID(tx)
	require.NoError(t, err)
	assert.Equal(t, "bucket-1", bucketID)

	updateSeq, err := GetBucketUpdateSeq(tx)
	require.NoError(t, err)
	assert.Equal(t, 3, updateSeq)

	require.NoError(t, SetBucketUpdateSeq(tx, 4))
	updateSeq, err = GetBucketUpdateSeq(tx)
	require.NoError(t, err)
	assert.Equal(t, 4, updateSeq)

	maxSeq, err := GetMaxDeploymentSeq(tx)
	require.NoError(t, err)
	assert.Equal(t, 1, maxSeq)

	count, err := CountAllocations(tx, true)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	assert.Equal(t, []string{"maand/job/api", "vars/bucket/job/api"}, BuildJobKVNamespaces("api"))
	assert.Equal(t, []string{"vars/job/api", "secrets/job/api"}, JobCommandKVNamespaces("api"))
	assert.Len(t, JobKVNamespaces("api"), 4)
}

func TestHashHelpers(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	require.NoError(t, UpdateHash(tx, "api_allocation", "alloc-1", "hash-new"))
	changed, err := HashChanged(tx, "api_allocation", "alloc-1")
	require.NoError(t, err)
	assert.True(t, changed)

	require.NoError(t, PromoteHash(tx, "api_allocation", "alloc-1"))
	changed, err = HashChanged(tx, "api_allocation", "alloc-1")
	require.NoError(t, err)
	assert.False(t, changed)

	prev, err := GetPreviousHash(tx, "api_allocation", "alloc-1")
	require.NoError(t, err)
	assert.Equal(t, "hash-new", prev)

	require.NoError(t, RemoveHash(tx, "api_allocation", "alloc-1"))
	prev, err = GetPreviousHash(tx, "api_allocation", "alloc-1")
	require.NoError(t, err)
	assert.Equal(t, "", prev)
}
