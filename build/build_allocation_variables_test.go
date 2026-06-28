// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"maand/kv"
)

func TestSyncAllocationKeyValuesPreservesCertKeys(t *testing.T) {
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, tx.Commit())

	kv.GetKVStore().Put("maand/job/app/worker/10.0.0.1", "certs/tls.crt", "CRT", 0)
	kv.GetKVStore().Put("maand/job/app/worker/10.0.0.1", "certs/tls.key", "KEY", 0)
	kv.GetKVStore().Put("maand/job/app/worker/10.0.0.1", "stale_meta", "old", 0)
	kv.GetKVStore().Put("maand/job/app/worker/10.0.0.1", "version", "1.0.0", 0)

	require.NoError(t, syncAllocationKeyValues("maand/job/app/worker/10.0.0.1", map[string]string{
		"app_allocation_index": "0",
	}))

	keys, err := kv.GetKVStore().GetKeys("maand/job/app/worker/10.0.0.1")
	require.NoError(t, err)
	assert.Contains(t, keys, "certs/tls.crt")
	assert.Contains(t, keys, "certs/tls.key")
	assert.Contains(t, keys, "app_allocation_index")
	assert.Contains(t, keys, "version")
	assert.NotContains(t, keys, "stale_meta")
}

func TestSyncCertKeyValuesPreservesAllocationMetadata(t *testing.T) {
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, tx.Commit())

	namespace := "maand/job/zookeeper/worker/10.48.200.1"
	store := kv.GetKVStore()
	store.Put(namespace, "certs/quorum.crt", "CRT", 0)
	store.Put(namespace, "certs/quorum.key", "KEY", 0)
	store.Put(namespace, "zookeeper_allocation_index", "0", 0)
	store.Put(namespace, "peer_workers", "10.48.200.2,10.48.200.3", 0)

	idxBefore, err := store.Get(namespace, "zookeeper_allocation_index")
	require.NoError(t, err)
	peersBefore, err := store.Get(namespace, "peer_workers")
	require.NoError(t, err)

	require.NoError(t, syncCertKeyValues(namespace, map[string]string{
		"certs/quorum.crt": "CRT",
		"certs/quorum.key": "KEY",
	}))

	idxAfter, err := store.Get(namespace, "zookeeper_allocation_index")
	require.NoError(t, err)
	assert.Equal(t, idxBefore.Version, idxAfter.Version)
	assert.Equal(t, "0", idxAfter.Value)

	peersAfter, err := store.Get(namespace, "peer_workers")
	require.NoError(t, err)
	assert.Equal(t, peersBefore.Version, peersAfter.Version)
	assert.Equal(t, "10.48.200.2,10.48.200.3", peersAfter.Value)
}

func TestBuildJobAllocationVariables(t *testing.T) {
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 0);
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '1024', '2000', 0),
		       ('w2', '10.0.0.2', '1024', '2000', 1);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'api', 0, 0, 0, '0.0.0'),
		       ('a2', '10.0.0.2', 'api', 0, 0, 0, '0.0.0');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, BuildJobAllocationVariables(tx, nil))
	require.NoError(t, tx.Commit())

	primaryNS := "maand/job/api/worker/10.0.0.1"
	store := kv.GetKVStore()

	idx, err := store.Get(primaryNS, "api_allocation_index")
	require.NoError(t, err)
	assert.Equal(t, "0", idx.Value)

	peers, err := store.Get(primaryNS, "peer_workers")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.2", peers.Value)
}

func TestBuildJobAllocationVariablesSyncsDisabledWorker(t *testing.T) {
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 0);
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '1024', '2000', 0),
		       ('w2', '10.0.0.2', '1024', '2000', 1);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'api', 0, 0, 0, '0.0.0'),
		       ('a2', '10.0.0.2', 'api', 1, 0, 0, '0.0.0');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.2", "api_allocation_index", "1", 0)
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.2", "stale_meta", "old", 0)
	require.NoError(t, syncJobAllocationVariables(tx, "api"))
	require.NoError(t, tx.Commit())

	disabledNS := "maand/job/api/worker/10.0.0.2"
	idx, err := kv.GetKVStore().Get(disabledNS, "api_allocation_index")
	require.NoError(t, err)
	assert.Equal(t, "1", idx.Value)

	peers, err := kv.GetKVStore().Get(disabledNS, "peer_workers")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", peers.Value)

	_, err = kv.GetKVStore().Get(disabledNS, "stale_meta")
	assert.Error(t, err)

	idx, err = kv.GetKVStore().Get("maand/job/api/worker/10.0.0.1", "api_allocation_index")
	require.NoError(t, err)
	assert.Equal(t, "0", idx.Value)
}

func TestBuildJobAllocationVariablesPurgeRemovedWorker(t *testing.T) {
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 0);
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '1024', '2000', 0),
		       ('w2', '10.0.0.2', '1024', '2000', 1);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'api', 0, 0, 0, '0.0.0'),
		       ('a2', '10.0.0.2', 'api', 0, 1, 0, '0.0.0');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.2", "api_allocation_index", "1", 0)
	require.NoError(t, syncJobAllocationVariables(tx, "api"))
	require.NoError(t, tx.Commit())

	keys, err := kv.GetKVStore().GetKeys("maand/job/api/worker/10.0.0.2")
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestBuildJobAllocationVariablesClearsRemovedJobWorkers(t *testing.T) {
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 0);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-old', 'legacy', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'legacy', 0, 0, 0, '0.0.0');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	kv.GetKVStore().Put("maand/job/legacy/worker/10.0.0.1", "legacy_allocation_index", "0", 0)
	require.NoError(t, BuildJobAllocationVariables(tx, []string{"legacy"}))
	require.NoError(t, tx.Commit())

	keys, err := kv.GetKVStore().GetKeys("maand/job/legacy/worker/10.0.0.1")
	require.NoError(t, err)
	assert.Empty(t, keys)
}
