// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetVersionPendingAllocations(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	active, err := GetVersionPendingAllocations(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1"}, active)

	all, err := GetVersionPendingNonRemovedAllocations(tx, "api")
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1"}, all)

	needs, err := AllocationNeedsVersionRollout(tx, "api", "10.0.0.1")
	require.NoError(t, err)
	assert.True(t, needs)

	needs, err = AllocationNeedsVersionRollout(tx, "api", "10.0.0.2")
	require.NoError(t, err)
	assert.False(t, needs)
}

func TestGetAllocationNewVersionDefaultsMissingRow(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	version, err := GetAllocationNewVersion(tx, "missing")
	require.NoError(t, err)
	assert.Equal(t, DefaultAllocationVersion, version)
}

func TestSetAllocationNewVersionNormalizesUnknown(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, SetAllocationNewVersion(tx, "alloc-1", "unknown"))
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	version, err := GetAllocationNewVersion(tx, "alloc-1")
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	assert.Equal(t, DefaultAllocationVersion, version)
}

func TestMarkAllocationStartPendingClearsPreviousHash(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, MarkAllocationStartPending(tx, "api", "alloc-1"))
	require.NoError(t, tx.Commit())

	var previousHash sql.NullString
	require.NoError(t, db.QueryRow(
		`SELECT previous_hash FROM hash WHERE namespace = 'api_allocation' AND key = 'alloc-1'`,
	).Scan(&previousHash))
	assert.False(t, previousHash.Valid)
}

func TestRemoveAllocationHash(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, RemoveAllocationHash(tx, "api", "alloc-1"))
	require.NoError(t, tx.Commit())

	var count int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM hash WHERE namespace = 'api_allocation' AND key = 'alloc-1'`,
	).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestTargetJobVersion(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	version, err := TargetJobVersion(tx, "api")
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	assert.Equal(t, "1.2.3", version)
}
