// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsActiveAllocation(t *testing.T) {
	assert.True(t, IsActiveAllocation(0, 0))
	assert.False(t, IsActiveAllocation(1, 0))
	assert.False(t, IsActiveAllocation(0, 1))
}

func TestIsAllocationActive(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	active, err := IsAllocationActive(tx, "10.0.0.1", "api")
	require.NoError(t, err)
	assert.True(t, active)

	active, err = IsAllocationActive(tx, "10.0.0.2", "api")
	require.NoError(t, err)
	assert.False(t, active)
}

func TestStoppedAllocationAssumeDead(t *testing.T) {
	catalog := NewWorkerCatalog([]string{"10.0.0.1"})
	assert.False(t, StoppedAllocationAssumeDead(StoppedAllocation{
		WorkerIP: "10.0.0.1", Removed: true,
	}, catalog))
	assert.True(t, StoppedAllocationAssumeDead(StoppedAllocation{
		WorkerIP: "10.0.0.2", Removed: true,
	}, catalog))
	assert.False(t, StoppedAllocationAssumeDead(StoppedAllocation{
		WorkerIP: "10.0.0.2", Disabled: true,
	}, catalog))
}
