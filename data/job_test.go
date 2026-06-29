// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func openJobTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:jobtest?mode=memory&cache=shared")
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, MigrateSchema(tx))
	require.NoError(t, tx.Commit())
	return db
}

func TestGetJobsByDeploymentSeqSkipsRemovedOrDeletedJobs(t *testing.T) {
	db := openJobTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)

	_, err = tx.Exec(`INSERT INTO job (job_id, name, version, min_memory_mb, max_memory_mb, current_memory_mb,
		min_cpu_mhz, max_cpu_mhz, current_cpu_mhz, max_concurrent_upgrades)
		VALUES ('job-active', 'active', '1', '128', '256', '128', '100', '200', '100', 1)`)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('alloc-active', '10.0.0.1', 'active', 0, 0, 0)`)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('alloc-removed', '10.0.0.2', 'removed', 0, 1, 0)`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	jobs, err := GetJobsByDeploymentSeq(tx, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"active"}, jobs)
}
