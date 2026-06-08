// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gc

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"maand/data"
	"maand/kv"

	_ "github.com/mattn/go-sqlite3"
)

func TestPurgeRemovedAllocationReferences(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:gcreftest?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	_, err = db.Exec(`
		CREATE TABLE worker (worker_ip TEXT PRIMARY KEY, position INT);
		CREATE TABLE allocations (
			alloc_id TEXT, worker_ip TEXT, job TEXT,
			disabled INT, removed INT, deployment_seq INT
		);
		CREATE TABLE key_value (
			key TEXT, value TEXT, namespace TEXT, version INT,
			ttl INT, created_date INT, deleted INT
		);
	`)
	require.NoError(t, err)
	suggestedWorker := "10.0.0.1"
	_, err = db.Exec(`INSERT INTO worker (worker_ip, position) VALUES (?, 0)`, suggestedWorker)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	store := kv.NewStore()
	store.Put("maand/job/app/worker/10.0.0.2", "cert", "pem", 0)
	store.Put("maand/job/app/worker/10.0.0.2", "port", "8080", 0)
	store.Put("maand/worker/10.0.0.2", "hostname", "w2", 0)
	store.Put("maand/worker/10.0.0.2/tags", "role", "web", 0)
	store.Put("maand/worker/10.0.0.1", "hostname", "w1", 0)

	allocs := []removedAllocation{
		{Job: "app", WorkerIP: "10.0.0.2"},
	}

	require.NoError(t, purgeRemovedAllocationReferences(tx, store, allocs))
	require.NoError(t, tx.Commit())

	keys, err := store.GetKeys("maand/job/app/worker/10.0.0.2")
	require.NoError(t, err)
	assert.Empty(t, keys)

	keys, err = store.GetKeys("maand/worker/10.0.0.2")
	require.NoError(t, err)
	assert.Empty(t, keys)

	keys, err = store.GetKeys("maand/worker/10.0.0.2/tags")
	require.NoError(t, err)
	assert.Empty(t, keys)

	keys, err = store.GetKeys("maand/worker/10.0.0.1")
	require.NoError(t, err)
	assert.Len(t, keys, 1)
}

func TestPurgeRemovedJobNamespacesWhenNoActiveAllocations(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:gcjobkv?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	_, err = db.Exec(`
		CREATE TABLE allocations (
			alloc_id TEXT, worker_ip TEXT, job TEXT,
			disabled INT, removed INT, deployment_seq INT
		);
		CREATE TABLE key_value (
			key TEXT, value TEXT, namespace TEXT, version INT,
			ttl INT, created_date INT, deleted INT
		);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('a1', '10.0.0.1', 'vault', 0, 1, 0),
		       ('a2', '10.0.0.2', 'vault', 0, 1, 0)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	store := kv.NewStore()
	store.Put("vars/job/vault", "cluster_initialized", "true", 0)
	store.Put("secrets/job/vault", "root_token", "enc:v1:abc", 0)
	store.Put("maand/job/vault", "version", "1.0.0", 0)
	store.Put("vars/bucket/job/vault", "memory", "512", 0)

	allocs := []removedAllocation{
		{Job: "vault", WorkerIP: "10.0.0.1"},
		{Job: "vault", WorkerIP: "10.0.0.2"},
	}

	require.NoError(t, purgeRemovedJobNamespaces(tx, store, allocs))

	for _, namespace := range data.JobKVNamespaces("vault") {
		keys, err := store.GetKeys(namespace)
		require.NoError(t, err)
		assert.Empty(t, keys, namespace)
	}
}

func TestPurgeRemovedJobNamespacesKeepsKVWhenJobStillActive(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:gcjobkvactive?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	_, err = db.Exec(`
		CREATE TABLE allocations (
			alloc_id TEXT, worker_ip TEXT, job TEXT,
			disabled INT, removed INT, deployment_seq INT
		);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('a1', '10.0.0.1', 'vault', 0, 1, 0),
		       ('a2', '10.0.0.2', 'vault', 0, 0, 0)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	store := kv.NewStore()
	store.Put("vars/job/vault", "cluster_initialized", "true", 0)
	store.Put("secrets/job/vault", "root_token", "enc:v1:abc", 0)

	allocs := []removedAllocation{{Job: "vault", WorkerIP: "10.0.0.1"}}

	require.NoError(t, purgeRemovedJobNamespaces(tx, store, allocs))

	keys, err := store.GetKeys("vars/job/vault")
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	keys, err = store.GetKeys("secrets/job/vault")
	require.NoError(t, err)
	assert.Len(t, keys, 1)
}
