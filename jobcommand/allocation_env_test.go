package jobcommand

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"maand/data"
	"maand/kv"

	_ "github.com/mattn/go-sqlite3"
)

func openAllocationEnvTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, data.MigrateSchema(tx))
	require.NoError(t, tx.Commit())
	return db
}

func seedTwoWorkerJob(t *testing.T, tx *sql.Tx) {
	t.Helper()
	_, err := tx.Exec(`
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
}

func TestBuildCommandEnvIncludesAllocationIndex(t *testing.T) {
	env := buildCommandEnv("alloc-1", "api", "10.0.0.1", "2", 0, "pre_deploy", "pre_deploy", nil)
	assert.Contains(t, env, "ALLOCATION_INDEX=2")
	assert.Contains(t, env, "ALLOCATION_ID=alloc-1")
	assert.Contains(t, env, "ALLOCATION_IP=10.0.0.1")
}

func TestAllocationIndexForJobCommand_fromKV(t *testing.T) {
	t.Cleanup(kv.ResetStoreForTest)

	db := openAllocationEnvTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedTwoWorkerJob(t, tx)
	require.NoError(t, kv.Initialize(tx))
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.2", "api_allocation_index", "1", 0)

	index, err := allocationIndexForJobCommand(tx, "api", "10.0.0.2")
	require.NoError(t, err)
	assert.Equal(t, "1", index)
	require.NoError(t, tx.Rollback())
}

func TestAllocationIndexForJobCommand_fromDBOrder(t *testing.T) {
	t.Cleanup(kv.ResetStoreForTest)

	db := openAllocationEnvTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedTwoWorkerJob(t, tx)
	require.NoError(t, kv.Initialize(tx))

	index, err := allocationIndexForJobCommand(tx, "api", "10.0.0.2")
	require.NoError(t, err)
	assert.Equal(t, "1", index)
	require.NoError(t, tx.Rollback())
}
