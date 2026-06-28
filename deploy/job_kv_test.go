// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPurgeJobCommandKVForInactiveJobs(t *testing.T) {
	env := setupDeployTestEnv(t)
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, _ string, _ bucket.CommandContext, _ []string, _ []string) error { return nil },
		Rsync:         func(*bucket.Runtime, string, string, []string) error { return nil },
		SetupRuntime:  func(string, bucket.RunContext) (*bucket.Runtime, error) { return nil, nil },
	})
	t.Cleanup(ClearTestHooks)

	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	store.Put("vars/job/vault", "cluster_initialized", "true", 0)
	store.Put("secrets/job/vault", "root_token", "enc:v1:token", 0)
	store.Put("maand/job/vault", "version", "1.0.0", 0)

	env.insertAllocation(t, tx, "alloc-1", "10.0.0.1", "vault", 0, 0, 1)
	env.insertAllocation(t, tx, "alloc-2", "10.0.0.2", "vault", 0, 0, 1)

	require.NoError(t, purgeJobCommandKVForInactiveJobs(tx, []data.StoppedAllocation{
		{WorkerIP: "10.0.0.1", Job: "vault", Removed: true},
		{WorkerIP: "10.0.0.2", Job: "vault", Removed: true},
	}))

	varsKeys, err := store.GetKeys("vars/job/vault")
	require.NoError(t, err)
	assert.Empty(t, varsKeys)
	secretKeys, err := store.GetKeys("secrets/job/vault")
	require.NoError(t, err)
	assert.Empty(t, secretKeys)

	maandKeys, err := store.GetKeys("maand/job/vault")
	require.NoError(t, err)
	assert.Empty(t, maandKeys)

	require.NoError(t, tx.Rollback())
}

func TestPurgeJobCommandKVSkipsWhenJobStillActive(t *testing.T) {
	env := setupDeployTestEnv(t)

	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	store.Put("vars/job/vault", "cluster_initialized", "true", 0)

	env.insertAllocation(t, tx, "alloc-1", "10.0.0.1", "vault", 0, 0, 1)
	env.insertAllocation(t, tx, "alloc-2", "10.0.0.2", "vault", 0, 0, 0)

	require.NoError(t, purgeJobCommandKVForInactiveJobs(tx, []data.StoppedAllocation{
		{WorkerIP: "10.0.0.1", Job: "vault", Removed: true},
	}))

	keys, err := store.GetKeys("vars/job/vault")
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	require.NoError(t, tx.Rollback())
}

func TestPurgeJobCommandKVSkipsWhenJobFullyDisabled(t *testing.T) {
	env := setupDeployTestEnv(t)

	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	store.Put("vars/job/vault", "cluster_initialized", "true", 0)

	env.insertAllocation(t, tx, "alloc-1", "10.0.0.1", "vault", 0, 1, 0)

	require.NoError(t, purgeJobCommandKVForInactiveJobs(tx, []data.StoppedAllocation{
		{WorkerIP: "10.0.0.1", Job: "vault", Disabled: true},
	}))

	keys, err := store.GetKeys("vars/job/vault")
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	require.NoError(t, tx.Rollback())
}
