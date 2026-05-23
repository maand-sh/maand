package deploy

import (
	"database/sql"
	"os"
	"path"
	"testing"

	"maand/bucket"
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
	require.NoError(t, updateAllocationHash(tx, []string{"app"}))

	var currentVersion, newVersion sql.NullString
	err = tx.QueryRow(
		`SELECT current_version, new_version FROM hash WHERE namespace = 'app_allocation' AND key = 'alloc-app-10.0.0.1'`,
	).Scan(&currentVersion, &newVersion)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0", currentVersion.String)
	assert.Equal(t, "2.0.0", newVersion.String)

	require.NoError(t, promoteAllocationHash(tx, "app"))
	assert.True(t, env.allocationHashPromoted(t, tx, "app", "alloc-app-10.0.0.1"))

	err = tx.QueryRow(
		`SELECT current_version, new_version FROM hash WHERE namespace = 'app_allocation' AND key = 'alloc-app-10.0.0.1'`,
	).Scan(&currentVersion, &newVersion)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", currentVersion.String)
	assert.Equal(t, "2.0.0", newVersion.String)
	require.NoError(t, tx.Rollback())
}

func TestUpdateJobAllocationHashes_removesDeletedAllocation(t *testing.T) {
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

	var allocCount, hashCount int
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM allocations WHERE job = 'gone'`).Scan(&allocCount))
	require.NoError(t, tx.QueryRow(
		`SELECT count(*) FROM hash WHERE namespace = 'gone_allocation' AND key = 'alloc-gone'`,
	).Scan(&hashCount))
	assert.Equal(t, 1, allocCount)
	assert.Equal(t, 0, hashCount)
	require.NoError(t, tx.Rollback())
}

func TestPromoteHash_skipsDisabledAllocation(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.ensureWorker(t, tx, "10.0.0.1", 0)
	jobID := env.insertJob(t, tx, "app", 0, 1)
	env.insertAllocation(t, tx, "alloc-app", "10.0.0.1", "app", 0, 1, 0)
	env.setAllocationHash(t, tx, "app", "alloc-app", "cur", "prev")
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, promoteAllocationHash(tx, "app"))
	var previousHash sql.NullString
	var currentVersion sql.NullString
	err := tx.QueryRow(
		`SELECT previous_hash, current_version FROM hash WHERE namespace = 'app_allocation' AND key = 'alloc-app'`,
	).Scan(&previousHash, &currentVersion)
	require.NoError(t, err)
	assert.False(t, previousHash.Valid)
	assert.False(t, currentVersion.Valid)
	_ = jobID
	require.NoError(t, tx.Rollback())
}
