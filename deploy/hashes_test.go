package deploy

import (
	"database/sql"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateJobAllocationHashes_andPromote(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	_, err := tx.Exec(`UPDATE job SET version = '2.0.0' WHERE name = 'app'`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, prepareJobsFiles(tx, []string{"app"}))
	_, err = tx.Exec(`UPDATE allocations SET new_version = '2.0.0' WHERE alloc_id = 'alloc-app-10.0.0.1'`)
	require.NoError(t, err)
	require.NoError(t, updateAllocationHash(tx, []string{"app"}))

	var currentVersion sql.NullString
	var newVersion sql.NullString
	err = tx.QueryRow(
		`SELECT current_version FROM hash WHERE namespace = 'app_allocation' AND key = 'alloc-app-10.0.0.1'`,
	).Scan(&currentVersion)
	require.NoError(t, err)
	err = tx.QueryRow(
		`SELECT new_version FROM allocations WHERE alloc_id = 'alloc-app-10.0.0.1'`,
	).Scan(&newVersion)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0", currentVersion.String)
	assert.Equal(t, "2.0.0", newVersion.String)
	assertAllocationVersionKV(t, "app", "10.0.0.1", "2.0.0")

	require.NoError(t, promoteAllocationHash(tx, "app"))
	assert.True(t, env.allocationHashPromoted(t, tx, "app", "alloc-app-10.0.0.1"))

	err = tx.QueryRow(
		`SELECT current_version FROM hash WHERE namespace = 'app_allocation' AND key = 'alloc-app-10.0.0.1'`,
	).Scan(&currentVersion)
	require.NoError(t, err)
	err = tx.QueryRow(
		`SELECT new_version FROM allocations WHERE alloc_id = 'alloc-app-10.0.0.1'`,
	).Scan(&newVersion)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", currentVersion.String)
	assert.Equal(t, "2.0.0", newVersion.String)
	assertAllocationVersionKV(t, "app", "10.0.0.1", "2.0.0")
	require.NoError(t, tx.Rollback())
}

func assertAllocationVersionKV(t *testing.T, job, workerIP, wantVersion string) {
	t.Helper()
	namespace := "maand/job/" + job + "/worker/" + workerIP
	store := kv.GetKVStore()
	entry, err := store.Get(namespace, "version")
	require.NoError(t, err)
	assert.Equal(t, wantVersion, entry.Value)
	for _, legacyKey := range []string{"current_version", "new_version"} {
		_, err := store.Get(namespace, legacyKey)
		assert.ErrorIs(t, err, kv.ErrNotFound, legacyKey)
	}
}

func TestSyncAllocationVersionKV_removesLegacyKeys(t *testing.T) {
	t.Cleanup(kv.ResetStoreForTest)

	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, tx.Commit())

	namespace := "maand/job/app/worker/10.0.0.1"
	store := kv.GetKVStore()
	store.Put(namespace, "current_version", "1.0.0", 0)
	store.Put(namespace, "new_version", "2.0.0", 0)

	require.NoError(t, syncAllocationVersionKV("app", "10.0.0.1", data.AllocationVersions{
		CurrentVersion: "1.0.0",
		NewVersion:     "2.0.0",
	}))
	assertAllocationVersionKV(t, "app", "10.0.0.1", "2.0.0")
}

func TestUpdateJobAllocationHashes_skipsRemovedAllocation(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.ensureWorker(t, tx, "10.0.0.1", 0)
	jobID := env.insertJob(t, tx, "gone", 0, 1)
	env.insertAllocation(t, tx, "alloc-gone", "10.0.0.1", "gone", 0, 0, 1)
	env.setAllocationHash(t, tx, "gone", "alloc-gone", "x", "y")
	env.insertJobFile(t, tx, jobID, path.Join("gone", "Makefile"), makefileContent(), false)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	workerDir := bucket.GetTempWorkerPath("10.0.0.1")
	require.NoError(t, os.MkdirAll(path.Join(workerDir, "jobs", "gone"), 0o755))
	require.NoError(t, updateAllocationHash(tx, []string{"gone"}))

	var hashCount int
	require.NoError(t, tx.QueryRow(
		`SELECT count(*) FROM hash WHERE namespace = 'gone_allocation' AND key = 'alloc-gone'`,
	).Scan(&hashCount))
	assert.Equal(t, 1, hashCount)
	require.NoError(t, tx.Rollback())
}

func TestUpdateJobAllocationHashes_multipleWorkers(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.ensureWorker(t, tx, "10.0.0.2", 1)
	jobID := env.insertJob(t, tx, "app", 0, 1)
	env.insertAllocation(t, tx, "alloc-app-10.0.0.1", "10.0.0.1", "app", 0, 0, 0)
	env.insertAllocation(t, tx, "alloc-app-10.0.0.2", "10.0.0.2", "app", 0, 0, 0)
	env.insertJobFile(t, tx, jobID, path.Join("app", "Makefile"), makefileContent(), false)
	require.NoError(t, tx.Commit())

	for _, workerIP := range []string{"10.0.0.1", "10.0.0.2"} {
		workerDir := bucket.GetTempWorkerPath(workerIP)
		require.NoError(t, os.MkdirAll(path.Join(workerDir, "jobs", "app"), 0o755))
	}

	tx = env.begin(t)
	require.NoError(t, updateJobAllocationHashes(tx, "app"))

	for _, allocID := range []string{"alloc-app-10.0.0.1", "alloc-app-10.0.0.2"} {
		var current sql.NullString
		err := tx.QueryRow(
			`SELECT current_hash FROM hash WHERE namespace = 'app_allocation' AND key = ?`,
			allocID,
		).Scan(&current)
		require.NoError(t, err)
		assert.True(t, current.Valid)
		assert.NotEmpty(t, current.String)
	}
	require.NoError(t, tx.Rollback())
}

func TestPromoteAllocationHash_multipleWorkers(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.ensureWorker(t, tx, "10.0.0.2", 1)
	jobID := env.insertJob(t, tx, "app", 0, 1)
	env.insertAllocation(t, tx, "alloc-app-10.0.0.1", "10.0.0.1", "app", 0, 0, 0)
	env.insertAllocation(t, tx, "alloc-app-10.0.0.2", "10.0.0.2", "app", 0, 0, 0)
	env.insertJobFile(t, tx, jobID, path.Join("app", "Makefile"), makefileContent(), false)
	require.NoError(t, tx.Commit())

	for _, workerIP := range []string{"10.0.0.1", "10.0.0.2"} {
		workerDir := bucket.GetTempWorkerPath(workerIP)
		require.NoError(t, os.MkdirAll(path.Join(workerDir, "jobs", "app"), 0o755))
	}

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, prepareJobsFiles(tx, []string{"app"}))
	require.NoError(t, updateAllocationHash(tx, []string{"app"}))
	require.NoError(t, promoteAllocationHash(tx, "app"))

	for _, allocID := range []string{"alloc-app-10.0.0.1", "alloc-app-10.0.0.2"} {
		assert.True(t, env.allocationHashPromoted(t, tx, "app", allocID))
	}
	require.NoError(t, tx.Rollback())
}

func TestPromoteHash_promotesDisabledAllocation(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	_, err := tx.Exec(`UPDATE allocations SET disabled = 1 WHERE alloc_id = 'alloc-app-10.0.0.1'`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, prepareJobsFiles(tx, []string{"app"}))
	require.NoError(t, updateAllocationHash(tx, []string{"app"}))
	require.NoError(t, promoteAllocationHash(tx, "app"))

	var previousHash sql.NullString
	err = tx.QueryRow(
		`SELECT previous_hash FROM hash WHERE namespace = 'app_allocation' AND key = 'alloc-app-10.0.0.1'`,
	).Scan(&previousHash)
	require.NoError(t, err)
	assert.True(t, previousHash.Valid)
	assert.NotEmpty(t, previousHash.String)
	require.NoError(t, tx.Rollback())
}
