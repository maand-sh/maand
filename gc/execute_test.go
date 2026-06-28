// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gc

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"maand/bucket"
	"maand/data"
	"maand/initialize"
	"maand/kv"
	"maand/worker"

	_ "github.com/mattn/go-sqlite3"
)

func TestListRemovedAllocationsMissingTable(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:gcmissing?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = listRemovedAllocations(tx)
	require.Error(t, err)
	require.NoError(t, tx.Rollback())
}

func TestListRemovedAllocationsEmpty(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:gcempty?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE TABLE allocations (alloc_id TEXT, worker_ip TEXT, job TEXT, disabled INT, removed INT, deployment_seq INT)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	allocs, err := listRemovedAllocations(tx)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	assert.Empty(t, allocs)
}

func TestListRemovedAllocations(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:gclist?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		CREATE TABLE allocations (alloc_id TEXT, worker_ip TEXT, job TEXT, disabled INT, removed INT, deployment_seq INT);
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('a1', '10.0.0.1', 'app', 0, 1, 0),
		       ('a2', '10.0.0.2', 'app', 0, 0, 0);
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	allocs, err := listRemovedAllocations(tx)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())

	require.Len(t, allocs, 1)
	assert.Equal(t, "10.0.0.1", allocs[0].WorkerIP)
	assert.Equal(t, "app", allocs[0].Job)
}

func TestExecutePurgesRemovedAllocationEndToEnd(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
		kv.ResetStoreForTest()
		worker.ClearTestHooks()
	})

	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '0', '0', 0);
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'app', 0, 1, 0, '0.0.0');
		INSERT INTO hash (namespace, key, current_hash) VALUES ('app_allocation', 'a1', 'hash');
		INSERT INTO key_value (namespace, key, value, version, ttl, created_date, deleted)
		VALUES ('maand/job/app/worker/10.0.0.1', 'cert', 'pem', 1, 0, 1, 0);
	`)
	require.NoError(t, err)

	worker.SetTestHooks(&worker.TestHooks{
		ExecuteCommand: func(_ *bucket.Runtime, _ string, _ bucket.CommandContext, _ []string, _ []string) error {
			return nil
		},
	})

	require.NoError(t, Execute(0))

	var allocCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM allocations`).Scan(&allocCount))
	assert.Equal(t, 0, allocCount)

	kvStore := kv.GetStore()
	require.NotNil(t, kvStore)
	keys, err := kvStore.GetKeys("maand/job/app/worker/10.0.0.1")
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestPurgeRemovedAllocationsMissingHashTable(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:gcpurgehash?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		CREATE TABLE allocations (alloc_id TEXT, worker_ip TEXT, job TEXT, disabled INT, removed INT, deployment_seq INT);
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('a1', '10.0.0.1', 'app', 0, 1, 0);
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	err = purgeRemovedAllocations(tx)
	require.Error(t, err)
	require.NoError(t, tx.Rollback())
}

func TestPurgeRemovedAllocations(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:gctest?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	_, err = db.Exec(`
		CREATE TABLE allocations (alloc_id TEXT, worker_ip TEXT, job TEXT, disabled INT, removed INT, deployment_seq INT);
		CREATE TABLE hash (namespace TEXT, key TEXT, current_hash TEXT, previous_hash TEXT, PRIMARY KEY(namespace, key));
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('a1', '10.0.0.1', 'app', 0, 1, 0),
		       ('a2', '10.0.0.2', 'app', 0, 1, 0);
		INSERT INTO hash (namespace, key, current_hash) VALUES
			('app_allocation', 'a1', 'hash1'),
			('app_allocation', 'a2', 'hash2');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, purgeRemovedAllocations(tx))
	require.NoError(t, tx.Commit())

	var allocCount, hashCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM allocations`).Scan(&allocCount))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM hash WHERE namespace = 'app_allocation'`).Scan(&hashCount))
	assert.Equal(t, 0, allocCount)
	assert.Equal(t, 0, hashCount)
}

func TestExecutePurgesMultipleRemovedAllocations(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
		kv.ResetStoreForTest()
		worker.ClearTestHooks()
	})

	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'app', 0, 1, 0, '0.0.0'),
		       ('a2', '10.0.0.2', 'app', 0, 1, 0, '0.0.0');
		INSERT INTO hash (namespace, key, current_hash) VALUES
			('app_allocation', 'a1', 'hash1'),
			('app_allocation', 'a2', 'hash2');
		INSERT INTO key_value (namespace, key, value, version, ttl, created_date, deleted)
		VALUES ('maand/job/app/worker/10.0.0.1', 'cert', 'pem1', 1, 0, 1, 0),
		       ('maand/job/app/worker/10.0.0.2', 'cert', 'pem2', 1, 0, 1, 0);
	`)
	require.NoError(t, err)

	worker.SetTestHooks(&worker.TestHooks{
		ExecuteCommand: func(_ *bucket.Runtime, _ string, _ bucket.CommandContext, _ []string, _ []string) error {
			return nil
		},
	})

	require.NoError(t, Execute(0))

	var allocCount, hashCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM allocations`).Scan(&allocCount))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM hash WHERE namespace = 'app_allocation'`).Scan(&hashCount))
	assert.Equal(t, 0, allocCount)
	assert.Equal(t, 0, hashCount)
}

func TestExecuteFailsWhenLiveWorkerCleanupFails(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
		kv.ResetStoreForTest()
		worker.ClearTestHooks()
	})

	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '0', '0', 0);
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'app', 0, 1, 0, '0.0.0');
	`)
	require.NoError(t, err)

	worker.SetTestHooks(&worker.TestHooks{
		ExecuteCommand: func(_ *bucket.Runtime, _ string, _ bucket.CommandContext, _ []string, _ []string) error {
			return assert.AnError
		},
	})

	err = Execute(0)
	require.Error(t, err)

	var allocCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM allocations`).Scan(&allocCount))
	assert.Equal(t, 1, allocCount)
}

func TestExecutePurgesStaleKVWithoutRemovedAllocations(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
		kv.ResetStoreForTest()
	})

	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	oldTS := time.Now().Add(-48 * time.Hour).Unix()
	_, err = db.Exec(`
		INSERT INTO key_value (namespace, key, value, version, ttl, created_date, deleted)
		VALUES ('vars/job/app', 'old', 'value', 1, 0, ?, 0),
		       ('vars/job/app', 'old', 'value', 2, 0, ?, 1);
	`, oldTS, oldTS)
	require.NoError(t, err)

	require.NoError(t, Execute(0))

	var rowCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM key_value WHERE namespace = 'vars/job/app'`).Scan(&rowCount))
	assert.Equal(t, 0, rowCount)
}

func TestCollectUsesDefaultRetention(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
		kv.ResetStoreForTest()
	})

	require.NoError(t, initialize.Execute())
	require.NoError(t, Collect())
}
