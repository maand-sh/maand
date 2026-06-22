// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"maand/bucket"
	"maand/kv"
	"maand/workspace"

	_ "github.com/mattn/go-sqlite3"
)

func openBuildVariablesTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	require.NoError(t, err)
	_, err = db.Exec(`
		CREATE TABLE allocations (
			alloc_id TEXT, worker_ip TEXT, job TEXT,
			disabled INT, removed INT, deployment_seq INT, new_version TEXT
		);
		CREATE TABLE key_value (
			key TEXT, value TEXT, namespace TEXT, version INT,
			ttl INT, created_date INT, deleted INT
		);
	`)
	require.NoError(t, err)
	return db
}

func TestJobShouldPurgeBuildKV(t *testing.T) {
	db := openBuildVariablesTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	purge, err := jobShouldPurgeBuildKV(tx, "missing")
	require.NoError(t, err)
	assert.False(t, purge)

	_, err = tx.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('a1', '10.0.0.1', 'vault', 1, 0, 0)`)
	require.NoError(t, err)

	purge, err = jobShouldPurgeBuildKV(tx, "vault")
	require.NoError(t, err)
	assert.False(t, purge)

	_, err = tx.Exec(`UPDATE allocations SET removed = 1 WHERE job = 'vault'`)
	require.NoError(t, err)

	purge, err = jobShouldPurgeBuildKV(tx, "vault")
	require.NoError(t, err)
	assert.True(t, purge)
}

func TestBuildJobVariablesSyncsDeployOrder(t *testing.T) {
	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '1024', '2000', 0),
		       ('w2', '10.0.0.2', '1024', '2000', 1);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, deploy_parallel_count, health_check
		) VALUES ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, 0, '');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'api', 0, 0, 0, '0.0.0'),
		       ('a2', '10.0.0.2', 'api', 0, 0, 0, '0.0.0');
	`)
	require.NoError(t, err)

	kv.ResetStoreForTest()
	t.Cleanup(kv.ResetStoreForTest)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, buildJobVariables(tx, nil, false))
	require.NoError(t, tx.Commit())

	order, err := kv.GetKVStore().Get("maand/job/api", "deploy_order")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1,10.0.0.2", order.Value)
}

func TestBuildJobVariablesPurgesCommandKVWhenRequested(t *testing.T) {
	db := openBuildVariablesTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		CREATE TABLE job (
			job_id TEXT, name TEXT, version TEXT,
			min_memory_mb TEXT, max_memory_mb TEXT, current_memory_mb TEXT,
			min_cpu_mhz TEXT, max_cpu_mhz TEXT, current_cpu_mhz TEXT,
			update_parallel_count INT, health_check TEXT,
			PRIMARY KEY(name)
		);
		CREATE TABLE job_selectors (job_id TEXT, selector TEXT);
		CREATE TABLE job_ports (job_id TEXT, name TEXT, port INT);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('a1', '10.0.0.1', 'api', 0, 1, 0);
	`)
	require.NoError(t, err)

	kv.ResetStoreForTest()
	t.Cleanup(kv.ResetStoreForTest)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, tx.Commit())

	store := kv.GetKVStore()
	store.Put("vars/job/api", "cluster_initialized", "true", 0)
	store.Put("secrets/job/api", "token", "secret", 0)

	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, buildJobVariables(tx, nil, true))
	require.NoError(t, tx.Commit())

	keys, err := store.GetKeys("vars/job/api")
	require.NoError(t, err)
	assert.Empty(t, keys)
	keys, err = store.GetKeys("secrets/job/api")
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestBuildJobVariablesPurgesRemovedJobKV(t *testing.T) {
	db := openBuildVariablesTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		CREATE TABLE job (
			job_id TEXT, name TEXT, version TEXT,
			min_memory_mb TEXT, max_memory_mb TEXT, current_memory_mb TEXT,
			min_cpu_mhz TEXT, max_cpu_mhz TEXT, current_cpu_mhz TEXT,
			update_parallel_count INT, health_check TEXT,
			PRIMARY KEY(name)
		);
		CREATE TABLE job_selectors (job_id TEXT, selector TEXT);
		CREATE TABLE job_ports (job_id TEXT, name TEXT, port INT);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-gone', 'gone', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('a1', '10.0.0.1', 'gone', 0, 1, 0);
	`)
	require.NoError(t, err)

	kv.ResetStoreForTest()
	t.Cleanup(kv.ResetStoreForTest)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, tx.Commit())

	store := kv.GetKVStore()
	store.Put("maand/job/gone", "version", "1.0.0", 0)
	store.Put("vars/bucket/job/gone", "memory", "512", 0)
	store.Put("vars/job/gone", "cluster_initialized", "true", 0)

	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, buildJobVariables(tx, nil, false))
	require.NoError(t, tx.Commit())

	for _, namespace := range []string{"maand/job/gone", "vars/bucket/job/gone"} {
		keys, err := store.GetKeys(namespace)
		require.NoError(t, err)
		assert.Empty(t, keys, namespace)
	}

	keys, err := store.GetKeys("vars/job/gone")
	require.NoError(t, err)
	assert.Len(t, keys, 1)
}

func TestSyncKeyValuesRemovesStaleKeys(t *testing.T) {
	db := openBuildVariablesTestDB(t)
	defer func() { _ = db.Close() }()

	kv.ResetStoreForTest()
	t.Cleanup(kv.ResetStoreForTest)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, tx.Commit())

	store := kv.GetKVStore()
	store.Put("maand/worker/10.0.0.1", "worker_ip", "10.0.0.1", 0)
	store.Put("maand/worker/10.0.0.1", "stale", "old", 0)

	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, syncKeyValues(tx, "maand/worker/10.0.0.1", map[string]string{
		"worker_ip": "10.0.0.1",
	}))
	require.NoError(t, tx.Commit())

	keys, err := store.GetKeys("maand/worker/10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, []string{"worker_ip"}, keys)
}

func TestLoadJobBucketConfig_readsPerJobSettings(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(`[api]
memory = "256 mb"
cpu = "200 mhz"
`), 0o644))

	name, settings, err := loadJobBucketConfig("api")
	require.NoError(t, err)
	assert.Equal(t, "bucket.jobs.conf", name)
	assert.Equal(t, "256 mb", settings["memory"])
	assert.Equal(t, "200 mhz", settings["cpu"])
}

func TestBuildBucketVariablesSyncsBucketConf(t *testing.T) {
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
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.conf"), []byte(`ssh_user = "deploy"`), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	_, err := db.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 1);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO job_ports (job_id, name, port) VALUES ('job-api', 'http_port', 9090);
	`)
	require.NoError(t, err)

	kv.ResetStoreForTest()
	t.Cleanup(kv.ResetStoreForTest)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, buildBucketVariables(tx))
	require.NoError(t, tx.Commit())

	store := kv.GetKVStore()
	sshUser, err := store.Get("vars/bucket", "ssh_user")
	require.NoError(t, err)
	assert.Equal(t, "deploy", sshUser.Value)

	httpPort, err := store.Get("maand", "http_port")
	require.NoError(t, err)
	assert.Equal(t, "9090", httpPort.Value)
}

func TestPurgeBuildJobKVNamespaces(t *testing.T) {
	kv.ResetStoreForTest()
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	_, err := db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'api', 0, 0, 0, '0.0.0');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, tx.Commit())

	store := kv.GetKVStore()
	store.Put("maand/job/api", "name", "api", 0)
	store.Put("maand/job/api/worker/10.0.0.1", "worker_ip", "10.0.0.1", 0)

	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, purgeBuildJobKVNamespaces(tx, "api"))
	require.NoError(t, tx.Commit())

	for _, namespace := range []string{"maand/job/api", "maand/job/api/worker/10.0.0.1"} {
		keys, err := store.GetKeys(namespace)
		require.NoError(t, err)
		assert.Empty(t, keys, namespace)
	}
}

func TestLoadJobBucketConfigMissingFile(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	name, settings, err := loadJobBucketConfig("api")
	require.NoError(t, err)
	assert.Equal(t, "bucket.jobs.conf", name)
	assert.NotNil(t, settings)
}

func TestBuildJobVariablesSyncsActiveJobKV(t *testing.T) {
	db := openBuildVariablesTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		CREATE TABLE job (
			job_id TEXT, name TEXT, version TEXT,
			min_memory_mb TEXT, max_memory_mb TEXT, current_memory_mb TEXT,
			min_cpu_mhz TEXT, max_cpu_mhz TEXT, current_cpu_mhz TEXT,
			update_parallel_count INT, health_check TEXT,
			PRIMARY KEY(name)
		);
		CREATE TABLE job_selectors (job_id TEXT, selector TEXT);
		CREATE TABLE job_ports (job_id TEXT, name TEXT, port INT);
		CREATE TABLE worker (worker_id TEXT, worker_ip TEXT PRIMARY KEY, available_memory_mb TEXT, available_cpu_mhz TEXT, position INT);
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '1024', '2000', 0);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-api', 'api', '1.0.0', '128', '256', '128', '100', '200', '100', 2, '');
		INSERT INTO job_selectors (job_id, selector) VALUES ('job-api', 'web');
		INSERT INTO job_ports (job_id, name, port) VALUES ('job-api', 'http_port', 8080);
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('a1', '10.0.0.1', 'api', 0, 0, 0);
	`)
	require.NoError(t, err)

	kv.ResetStoreForTest()
	t.Cleanup(kv.ResetStoreForTest)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, buildJobVariables(tx, nil, false))
	require.NoError(t, tx.Commit())

	store := kv.GetKVStore()
	keys, err := store.GetKeys("maand/job/api")
	require.NoError(t, err)
	assert.Contains(t, keys, "name")
	assert.Contains(t, keys, "workers")
}

func TestPurgeJobCommandKVNamespaces(t *testing.T) {
	kv.ResetStoreForTest()
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, tx.Commit())

	store := kv.GetKVStore()
	store.Put("vars/job/gone", "cluster_initialized", "true", 0)
	store.Put("secrets/job/gone", "token", "secret", 0)

	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, purgeJobCommandKVNamespaces(tx, "gone"))
	require.NoError(t, tx.Commit())

	for _, namespace := range []string{"vars/job/gone", "secrets/job/gone"} {
		keys, err := store.GetKeys(namespace)
		require.NoError(t, err)
		assert.Empty(t, keys, namespace)
	}
}

func TestMergeWorkspaceJobVars(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
		kv.ResetStoreForTest()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "vars.toml"), []byte(`name = "maand"`), 0o644))

	kv.ResetStoreForTest()
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildVariablesTestDB(t)
	defer func() { _ = db.Close() }()
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, tx.Commit())

	require.NoError(t, mergeWorkspaceJobVars("api"))

	value, err := kv.GetKVStore().Get("vars/job/api", "name")
	require.NoError(t, err)
	assert.Equal(t, "maand", value.Value)
}

func TestBuildVariablesPurgesRemovedWorkerKV(t *testing.T) {
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
	workersJSON := `[{"host":"10.0.0.1","memory":"1024","cpu":"2000","position":0}]`
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(workersJSON), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	_, err := db.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 1);
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '1024', '2000', 0);
	`)
	require.NoError(t, err)

	kv.ResetStoreForTest()
	t.Cleanup(kv.ResetStoreForTest)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	kv.GetKVStore().Put("maand/worker/10.0.0.99", "worker_ip", "10.0.0.99", 0)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, BuildVariables(tx, workspace.Default(), []string{"10.0.0.99"}, nil, false))
	require.NoError(t, tx.Commit())

	keys, err := kv.GetKVStore().GetKeys("maand/worker/10.0.0.99")
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestBuildVariablesSyncsWorkerLabelsAndPeers(t *testing.T) {
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
	workersJSON := `[
		{"host":"10.0.0.1","labels":["web"],"memory":"1024","cpu":"2000","position":0},
		{"host":"10.0.0.2","labels":["web"],"memory":"1024","cpu":"2000","position":1}
	]`
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(workersJSON), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	_, err := db.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 1);
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '1024', '2000', 0),
		       ('w2', '10.0.0.2', '1024', '2000', 1);
		INSERT INTO worker_labels (worker_id, label) VALUES ('w1', 'web'), ('w2', 'web');
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
	peers, err := store.Get("maand/worker/10.0.0.1", "web_peers")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.2", peers.Value)

	shared, err := store.Get("maand/worker", "web_workers")
	require.NoError(t, err)
	assert.Contains(t, shared.Value, "10.0.0.1")
	assert.Contains(t, shared.Value, "10.0.0.2")
}

func TestWorkerMetaByIP(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	workersJSON := `[{"host":"10.0.0.1","hostname":"worker-one","memory":"1024","cpu":"2000","position":0}]`
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(workersJSON), 0o644))

	meta, err := workerMetaByIP(workspace.Default())
	require.NoError(t, err)
	assert.Equal(t, "worker-one", meta["10.0.0.1"].Hostname)
}
