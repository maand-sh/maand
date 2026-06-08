package healthcheck

import (
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/initialize"

	"github.com/stretchr/testify/require"
)

func TestHealthCheckSkipsJobWithoutConfig(t *testing.T) {
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
	t.Cleanup(func() { _ = tx.Rollback() })

	_, err = tx.Exec(`
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count
		) VALUES ('id1', 'plain', '1', '0', '0', '0', '0', '0', '0', 1)`)
	require.NoError(t, err)

	rt, err := bucket.SetupRuntime("test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Stop() })

	require.NoError(t, HealthCheck(tx, rt, false, "plain", false))
}

func TestCheckWorkersSkipsWhenNoWorkers(t *testing.T) {
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
	t.Cleanup(func() { _ = tx.Rollback() })

	require.NoError(t, CheckWorkers(tx, false))
}
