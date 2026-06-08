// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"database/sql"
	"testing"

	"maand/data"
	"maand/gc"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGCPurgesJobKVWhenNoActiveAllocations(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"},{"host":"10.0.0.2"}]`)
	writeMinimalJob(t, "vault", `{"selectors":["worker"],"version":"1.0.0"}`)
	runBuild(t)

	seedJobKV(t, "vault", "vars/job/vault", "cluster_initialized", "true")
	seedJobKV(t, "vault", "secrets/job/vault", "root_token", "enc:v1:token")

	require.Equal(t, 2, MustQueryCount(t, `SELECT count(*) FROM allocations WHERE job = 'vault'`))
	require.NoError(t, withDatabase(func(db *sql.DB) error {
		_, err := db.Exec(`UPDATE allocations SET removed = 1 WHERE job = 'vault'`)
		return err
	}))
	// Off-catalog workers: GC assumes dead and does not fail SSH cleanup.
	require.NoError(t, withDatabase(func(db *sql.DB) error {
		_, err := db.Exec(`DELETE FROM worker`)
		return err
	}))

	require.NoError(t, gc.Execute(0))

	assertKVNamespaceDeleted(t, "vars/job/vault", "cluster_initialized")
	assertKVNamespaceDeleted(t, "secrets/job/vault", "root_token")
	assert.Equal(t, 0, MustQueryCount(t, `SELECT count(*) FROM allocations WHERE job = 'vault'`))
}

func TestGCKeepsJobKVWhenAnotherAllocationActive(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"},{"host":"10.0.0.2"}]`)
	writeMinimalJob(t, "vault", `{"selectors":["worker"],"version":"1.0.0"}`)
	runBuild(t)

	seedJobKV(t, "vault", "vars/job/vault", "cluster_initialized", "true")
	seedJobKV(t, "vault", "secrets/job/vault", "root_token", "enc:v1:token")

	require.NoError(t, withDatabase(func(db *sql.DB) error {
		_, err := db.Exec(`UPDATE allocations SET removed = 1 WHERE job = 'vault' AND worker_ip = '10.0.0.1'`)
		return err
	}))
	require.NoError(t, withDatabase(func(db *sql.DB) error {
		_, err := db.Exec(`DELETE FROM worker WHERE worker_ip = '10.0.0.1'`)
		return err
	}))

	require.NoError(t, gc.Execute(0))

	assert.Equal(t, "true", mustGetPersistedKV(t, "vars/job/vault", "cluster_initialized"))
	assert.Equal(t, "enc:v1:token", mustGetPersistedKV(t, "secrets/job/vault", "root_token"))
	assert.Equal(t, 1, MustQueryCount(t, `SELECT count(*) FROM allocations WHERE job = 'vault' AND removed = 0`))
}

func seedJobKV(t *testing.T, job, namespace, key, value string) {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() {
		_ = tx.Rollback()
	}()

	require.NoError(t, kv.Initialize(tx))
	kv.GetKVStore().Put(namespace, key, value, 0)
	require.NoError(t, kv.PersistSession())
	_ = job
}

func assertKVNamespaceDeleted(t *testing.T, namespace, key string) {
	t.Helper()
	var deleted int
	err := withDatabase(func(db *sql.DB) error {
		return db.QueryRow(`
			SELECT deleted FROM key_value
			WHERE namespace = ? AND key = ?
			ORDER BY version DESC LIMIT 1`, namespace, key).Scan(&deleted)
	})
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
}
