// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetJobHealthCheckUnset(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	spec, err := GetJobHealthCheck(tx, "api")
	require.NoError(t, err)
	assert.Nil(t, spec)

	spec, err = GetJobHealthCheck(tx, "missing")
	require.NoError(t, err)
	assert.Nil(t, spec)
}

func TestGetJobHealthCheckWithSpec(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	_, err = tx.Exec(`UPDATE job SET health_check = ? WHERE name = 'api'`,
		`{"checks":[{"type":"http","path":"/health"}]}`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	spec, err := GetJobHealthCheck(tx, "api")
	require.NoError(t, err)
	require.NotNil(t, spec)
	require.Len(t, spec.Checks, 1)
}

func TestGetJobHealthCheckInvalidJSON(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	_, err = tx.Exec(`UPDATE job SET health_check = 'not-json' WHERE name = 'api'`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	_, err = GetJobHealthCheck(tx, "api")
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidManifest)
}

func TestGetJobPortNumber(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	port, err := GetJobPortNumber(tx, "api", "http_port")
	require.NoError(t, err)
	assert.Equal(t, 8080, port)

	_, err = GetJobPortNumber(tx, "api", "missing")
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidManifest)
}
