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

func TestBucketInitializedEmpty(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:bucketempty?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE TABLE bucket (bucket_id TEXT, update_seq INT)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	initialized, err := BucketInitialized(tx)
	require.NoError(t, err)
	assert.False(t, initialized)
	require.NoError(t, tx.Rollback())
}

func TestInsertBucketRecordOnEmptyTable(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:bucketinsert?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE TABLE bucket (bucket_id TEXT, update_seq INT)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, InsertBucketRecord(tx, "bucket-new"))
	require.NoError(t, tx.Commit())

	var bucketID string
	require.NoError(t, db.QueryRow(`SELECT bucket_id FROM bucket`).Scan(&bucketID))
	assert.Equal(t, "bucket-new", bucketID)
}

func TestAccessibleKVNamespacesForJob(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:accessiblekv?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	_, err = db.Exec(`
		CREATE TABLE allocations (alloc_id TEXT, worker_ip TEXT, job TEXT, disabled INT, removed INT, deployment_seq INT);
		CREATE TABLE job_commands (
			job TEXT, name TEXT, executed_on TEXT,
			demand_job TEXT, demand_command TEXT, demand_config TEXT
		);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('a1', '10.0.0.1', 'vault', 0, 0, 0),
		       ('a2', '10.0.0.2', 'vault', 0, 0, 0)`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO job_commands (job, name, executed_on, demand_job, demand_command, demand_config)
		VALUES ('vault', 'init', 'pre_deploy', 'postgres', 'status', '')`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	namespaces, err := AccessibleKVNamespacesForJob(tx, "vault")
	require.NoError(t, err)

	assert.Contains(t, namespaces, "maand")
	assert.Contains(t, namespaces, "vars/bucket")
	assert.Contains(t, namespaces, "maand/worker")
	assert.Contains(t, namespaces, "maand/worker/10.0.0.1")
	assert.Contains(t, namespaces, "maand/worker/10.0.0.2")
	assert.Contains(t, namespaces, "maand/worker/10.0.0.1/tags")
	assert.Contains(t, namespaces, "vars/job/vault")
	assert.Contains(t, namespaces, "secrets/job/vault")
	assert.Contains(t, namespaces, "maand/job/vault/worker/10.0.0.1")
	assert.Contains(t, namespaces, "vars/job/postgres")
	assert.Contains(t, namespaces, "secrets/job/postgres")
	assert.NotContains(t, namespaces, "vars/job/api")
}

func TestUpstreamDemandKVNamespaces(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:upstreamdemand?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		CREATE TABLE job_commands (
			job TEXT, name TEXT, executed_on TEXT,
			demand_job TEXT, demand_command TEXT, demand_config TEXT
		);
		INSERT INTO job_commands (job, name, executed_on, demand_job, demand_command, demand_config)
		VALUES ('vault', 'init', 'pre_deploy', 'postgres', 'status', ''),
		       ('vault', 'backup', 'post_deploy', 'postgres', 'dump', ''),
		       ('vault', 'local', 'pre_deploy', '', '', '');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	namespaces, err := UpstreamDemandKVNamespaces(tx, "vault")
	require.NoError(t, err)
	assert.Contains(t, namespaces, "vars/job/postgres")
	assert.Contains(t, namespaces, "secrets/job/postgres")
	assert.Contains(t, namespaces, "maand/job/postgres")
	require.NoError(t, tx.Rollback())
}

func TestAllowedKVNamespacesWithUpstream(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:upstreamkv?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		CREATE TABLE job_commands (
			job TEXT, name TEXT, executed_on TEXT,
			demand_job TEXT, demand_command TEXT, demand_config TEXT
		);
		INSERT INTO job_commands (job, name, executed_on, demand_job, demand_command, demand_config)
		VALUES ('vault', 'init', 'pre_deploy', 'postgres', 'status', '');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	namespaces, err := AllowedKVNamespacesWithUpstream(tx, "vault", "10.0.0.1")
	require.NoError(t, err)
	assert.Contains(t, namespaces, "vars/job/postgres")
	assert.Contains(t, namespaces, "secrets/job/postgres")

	namespaces, err = AllowedKVNamespacesWithUpstream(nil, "vault", "10.0.0.1")
	require.NoError(t, err)
	assert.Contains(t, namespaces, "vars/job/vault")
	require.NoError(t, tx.Rollback())
}

func TestAccessibleKVNamespacesForJob_ignoresRemovedAllocations(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:accessiblekvremoved?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	_, err = db.Exec(`
		CREATE TABLE allocations (alloc_id TEXT, worker_ip TEXT, job TEXT, disabled INT, removed INT, deployment_seq INT);
		CREATE TABLE job_commands (
			job TEXT, name TEXT, executed_on TEXT,
			demand_job TEXT, demand_command TEXT, demand_config TEXT
		);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('a1', '10.0.0.1', 'vault', 0, 0, 0),
		       ('a2', '10.0.0.2', 'vault', 0, 1, 0)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	namespaces, err := AccessibleKVNamespacesForJob(tx, "vault")
	require.NoError(t, err)

	assert.Contains(t, namespaces, "maand/worker/10.0.0.1")
	assert.NotContains(t, namespaces, "maand/worker/10.0.0.2")
	assert.Contains(t, namespaces, "maand/job/vault/worker/10.0.0.1")
	assert.NotContains(t, namespaces, "maand/job/vault/worker/10.0.0.2")
}

func TestAccessibleKVNamespacesForJob_includesDisabledAllocations(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:accessiblekvdisabled?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	_, err = db.Exec(`
		CREATE TABLE allocations (alloc_id TEXT, worker_ip TEXT, job TEXT, disabled INT, removed INT, deployment_seq INT);
		CREATE TABLE job_commands (
			job TEXT, name TEXT, executed_on TEXT,
			demand_job TEXT, demand_command TEXT, demand_config TEXT
		);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('a1', '10.0.0.1', 'vault', 1, 0, 0)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	namespaces, err := AccessibleKVNamespacesForJob(tx, "vault")
	require.NoError(t, err)

	assert.Contains(t, namespaces, "vars/job/vault")
	assert.Contains(t, namespaces, "maand/job/vault/worker/10.0.0.1")
}
