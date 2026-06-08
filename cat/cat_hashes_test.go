package cat

import (
	"database/sql"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashesShowsAllocationHashState(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	tx, err := db.Begin()
	require.NoError(t, err)

	_, err = tx.Exec(`
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-vault', 'vault', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '')`)
	require.NoError(t, err)
	_, err = tx.Exec(`
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '0', '0', 0)`)
	require.NoError(t, err)
	_, err = tx.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('alloc-1', '10.0.0.1', 'vault', 0, 0, 0, '1.1.0')`)
	require.NoError(t, err)
	_, err = tx.Exec(`
		INSERT INTO hash (namespace, key, current_hash, previous_hash, current_version)
		VALUES ('vault_allocation', 'alloc-1', 'hash-new', 'hash-old', '1.0.0')`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	assert.Equal(t, "restart", rolloutStatus(0, 0, "hash-new", "hash-old", "1.0.0", "1.0.0"))
	assert.Equal(t, "promoted", rolloutStatus(0, 0, "same", "same", "1.0.0", "1.0.0"))
	assert.Equal(t, "restart", rolloutStatus(0, 0, "same", "same", "1.0.0", "2.0.0"))
	assert.Equal(t, "new", rolloutStatus(0, 0, "hash-new", "", "1.0.0", "1.0.0"))
	assert.Equal(t, "new", rolloutStatus(0, 0, "", "", "", ""))
	assert.Equal(t, "health_failed", rolloutStatus(0, 0, "hash-new", data.HealthFailedPreviousHash, "1.0.0", "1.0.0"))
	assert.Equal(t, "removed", rolloutStatus(0, 1, "", "", "", ""))
	assert.Equal(t, "disabled", rolloutStatus(1, 0, "same", "same", "1.0.0", "1.0.0"))
	assert.Equal(t, "disabled_restart", rolloutStatus(1, 0, "same", "same", "1.0.0", "2.0.0"))

	require.NoError(t, Hashes("vault", "10.0.0.1", false))
}

func TestHashesFiltersByJobAndWorker(t *testing.T) {
	env := setupHashesTestBucket(t)

	tx, err := env.db.Begin()
	require.NoError(t, err)
	seedHashAllocation(t, tx, "vault", "10.0.0.1", "alloc-1", "h1", "h0")
	seedHashAllocation(t, tx, "api", "10.0.0.2", "alloc-2", "h2", "h2")
	require.NoError(t, tx.Commit())

	require.NoError(t, Hashes("vault", "", false))
	require.NoError(t, Hashes("", "10.0.0.2", false))
	require.NoError(t, Hashes("vault", "10.0.0.1", false))
}

func TestHashesActiveFilterExcludesRemoved(t *testing.T) {
	env := setupHashesTestBucket(t)

	tx, err := env.db.Begin()
	require.NoError(t, err)
	seedHashAllocation(t, tx, "vault", "10.0.0.1", "alloc-1", "h1", "h0")
	_, err = tx.Exec(`
		UPDATE allocations SET removed = 1 WHERE alloc_id = 'alloc-1'`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	assert.Error(t, Hashes("vault", "", true))
	require.NoError(t, Hashes("vault", "", false))
}

func TestHashesRejectsUnknownFilter(t *testing.T) {
	env := setupHashesTestBucket(t)

	tx, err := env.db.Begin()
	require.NoError(t, err)
	seedHashAllocation(t, tx, "vault", "10.0.0.1", "alloc-1", "h1", "h0")
	require.NoError(t, tx.Commit())

	assert.Error(t, Hashes("missing", "", false))
	assert.Error(t, Hashes("", "10.0.0.99", false))
}

type hashesTestEnv struct {
	db *sql.DB
}

func setupHashesTestBucket(t *testing.T) hashesTestEnv {
	t.Helper()
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})
	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return hashesTestEnv{db: db}
}

func seedHashAllocation(t *testing.T, tx *sql.Tx, job, workerIP, allocID, currentHash, previousHash string) {
	t.Helper()
	_, err := tx.Exec(`
		INSERT OR IGNORE INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES (?, ?, '1.0.0', '0', '0', '0', '0', '0', '0', 1, '')`,
		"job-"+job, job,
	)
	require.NoError(t, err)
	_, err = tx.Exec(`
		INSERT OR IGNORE INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES (?, ?, '0', '0', 0)`, "w-"+workerIP, workerIP)
	require.NoError(t, err)
	_, err = tx.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES (?, ?, ?, 0, 0, 0, '0.0.0')`, allocID, workerIP, job)
	require.NoError(t, err)
	_, err = tx.Exec(`
		INSERT INTO hash (namespace, key, current_hash, previous_hash)
		VALUES (?, ?, ?, ?)`, job+"_allocation", allocID, currentHash, previousHash)
	require.NoError(t, err)
}
