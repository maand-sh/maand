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

func TestBoolString(t *testing.T) {
	assert.Equal(t, "1", boolString(true))
	assert.Equal(t, "0", boolString(false))
}

func TestEncodePeerPorts(t *testing.T) {
	portMap := map[string]string{
		"cql_port":  "9042",
		"http_port": "8080",
	}
	got := encodePeerPorts([]string{"10.0.0.2"}, portMap)
	assert.Equal(t, "10.0.0.2:cql_port:9042,10.0.0.2:http_port:8080", got)
	assert.Empty(t, encodePeerPorts(nil, portMap))
	assert.Empty(t, encodePeerPorts([]string{"10.0.0.2"}, nil))
}

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

	require.NoError(t, syncAllocationKeyValues("maand/job/app/worker/10.0.0.1", map[string]string{
		"app_allocation_index": "0",
	}))

	keys, err := kv.GetKVStore().GetKeys("maand/job/app/worker/10.0.0.1")
	require.NoError(t, err)
	assert.Contains(t, keys, "certs/tls.crt")
	assert.Contains(t, keys, "certs/tls.key")
	assert.Contains(t, keys, "app_allocation_index")
	assert.NotContains(t, keys, "stale_meta")
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
		INSERT INTO job_ports (job_id, name, port) VALUES ('job-api', 'http_port', 8080);
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
	secondaryNS := "maand/job/api/worker/10.0.0.2"
	store := kv.GetKVStore()

	idx, err := store.Get(primaryNS, "api_allocation_index")
	require.NoError(t, err)
	assert.Equal(t, "0", idx.Value)

	primary, err := store.Get(primaryNS, "is_primary")
	require.NoError(t, err)
	assert.Equal(t, "1", primary.Value)

	secondary, err := store.Get(secondaryNS, "is_primary")
	require.NoError(t, err)
	assert.Equal(t, "0", secondary.Value)

	peers, err := store.Get(primaryNS, "peer_workers")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.2", peers.Value)

	peerPorts, err := store.Get(primaryNS, "peer_ports")
	require.NoError(t, err)
	assert.Contains(t, peerPorts.Value, "10.0.0.2:http_port:8080")
}

func TestBuildJobAllocationVariablesPurgeStaleWorker(t *testing.T) {
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

	keys, err := kv.GetKVStore().GetKeys("maand/job/api/worker/10.0.0.2")
	require.NoError(t, err)
	assert.Empty(t, keys)

	idx, err := kv.GetKVStore().Get("maand/job/api/worker/10.0.0.1", "api_allocation_index")
	require.NoError(t, err)
	assert.Equal(t, "0", idx.Value)
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
