// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func openValidateTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	require.NoError(t, err)
	_, err = db.Exec(`
		CREATE TABLE worker (worker_ip TEXT PRIMARY KEY, available_memory_mb REAL, available_cpu_mhz REAL);
		CREATE TABLE job (name TEXT PRIMARY KEY, current_memory_mb REAL, current_cpu_mhz REAL);
		CREATE TABLE allocations (job TEXT, worker_ip TEXT, removed INT, disabled INT);
	`)
	require.NoError(t, err)
	return db
}

func TestValidateWorkerResources_requiresWorkerMemoryWhenJobNeedsMemory(t *testing.T) {
	db := openValidateTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO worker (worker_ip, available_memory_mb, available_cpu_mhz) VALUES ('10.0.0.1', 0, 0);
		INSERT INTO job (name, current_memory_mb, current_cpu_mhz) VALUES ('app', 512, 0);
		INSERT INTO allocations (job, worker_ip, removed, disabled) VALUES ('app', '10.0.0.1', 0, 0);
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = ValidateWorkerResources(tx)
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInsufficientResource)
	assert.Contains(t, err.Error(), "must specify memory")
}

func TestValidateWorkerResources_requiresWorkerCPUWhenJobNeedsCPU(t *testing.T) {
	db := openValidateTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO worker (worker_ip, available_memory_mb, available_cpu_mhz) VALUES ('10.0.0.1', 0, 0);
		INSERT INTO job (name, current_memory_mb, current_cpu_mhz) VALUES ('app', 0, 2000);
		INSERT INTO allocations (job, worker_ip, removed, disabled) VALUES ('app', '10.0.0.1', 0, 0);
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = ValidateWorkerResources(tx)
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInsufficientResource)
	assert.Contains(t, err.Error(), "must specify cpu")
}

func TestValidateWorkerResources_skipsWhenJobHasNoRequirements(t *testing.T) {
	db := openValidateTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO worker (worker_ip, available_memory_mb, available_cpu_mhz) VALUES ('10.0.0.1', 0, 0);
		INSERT INTO job (name, current_memory_mb, current_cpu_mhz) VALUES ('app', 0, 0);
		INSERT INTO allocations (job, worker_ip, removed, disabled) VALUES ('app', '10.0.0.1', 0, 0);
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	require.NoError(t, ValidateWorkerResources(tx))
}
