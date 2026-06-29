// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"maand/bucket"
	"maand/kv"
	"maand/workspace"
)

func TestBuildVariablesSyncsWorkerAndBucketKV(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
		kv.ResetStoreForTest()
	})

	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.MkdirAll(bucket.SecretLocation, 0o755))
	require.NoError(t, os.WriteFile(path.Join(bucket.SecretLocation, "ca.crt"), []byte("test-ca"), 0o644))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1","memory":"1024","cpu":"2000","position":0}]`), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 1);
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '1024', '2000', 0);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			max_concurrent_upgrades, health_check
		) VALUES ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, BuildVariables(tx, workspace.Default(), nil, nil, false))
	require.NoError(t, tx.Commit())

	store := kv.GetKVStore()
	keys, err := store.GetKeys("maand/worker/10.0.0.1")
	require.NoError(t, err)
	assert.Contains(t, keys, "worker_ip")

	keys, err = store.GetKeys("maand/bucket")
	require.NoError(t, err)
	assert.Contains(t, keys, "bucket_id")

	keys, err = store.GetKeys("maand/worker")
	require.NoError(t, err)
	assert.Contains(t, keys, "certs/ca.crt")
}

func TestBuildDeploymentSequenceOrdersJobs(t *testing.T) {
	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			max_concurrent_upgrades, health_check
		) VALUES ('job-b', 'b', '1.0.0', '0', '0', '0', '0', '0', '0', 1, ''),
		          ('job-a', 'a', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
		VALUES ('job-b', 'b', 'init', 'pre_deploy', 'a', 'status', ''),
		       ('job-a', 'a', 'init', 'pre_deploy', '', '', '');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'a', 0, 0, 0, '0.0.0'),
		       ('a2', '10.0.0.1', 'b', 0, 0, 0, '0.0.0');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, BuildDeploymentSequence(tx))
	require.NoError(t, tx.Commit())

	var seqA, seqB int
	require.NoError(t, db.QueryRow(`SELECT deployment_seq FROM allocations WHERE job = 'a'`).Scan(&seqA))
	require.NoError(t, db.QueryRow(`SELECT deployment_seq FROM allocations WHERE job = 'b'`).Scan(&seqB))
	assert.Less(t, seqA, seqB)
}
