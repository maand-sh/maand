package cat

import (
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCatCatalogBucket(t *testing.T) {
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

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 1);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '1024', '2000', 0);
		INSERT INTO worker_labels (worker_id, label) VALUES ('w1', 'web');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('alloc-1', '10.0.0.1', 'api', 0, 0, 0, '1.0.0');
		INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
		VALUES ('job-api', 'api', 'init', 'pre_deploy', '', '', '');
		INSERT INTO job_ports (job_id, name, port) VALUES ('job-api', 'http_port', 8080);
	`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
}

func TestCatCatalogCommands(t *testing.T) {
	setupCatCatalogBucket(t)

	captureStdout(t, func() {
		require.NoError(t, Jobs())
	})
	captureStdout(t, func() {
		require.NoError(t, Workers())
	})
	captureStdout(t, func() {
		require.NoError(t, Allocations("api", "10.0.0.1"))
	})
	captureStdout(t, func() {
		require.NoError(t, JobCommands())
	})
	captureStdout(t, func() {
		require.NoError(t, JobPorts())
	})
	captureStdout(t, func() {
		require.NoError(t, Info())
	})
}

func TestCatCatalogRejectsUnknownFilters(t *testing.T) {
	setupCatCatalogBucket(t)

	assert.Error(t, Allocations("missing", ""))
	assert.Error(t, Allocations("", "10.0.0.99"))
}

func TestParseCSVFilter(t *testing.T) {
	assert.Nil(t, parseCSVFilter(""))
	assert.Equal(t, []string{"a", "b"}, parseCSVFilter("a,b"))
	assert.Equal(t, []string{"a", "b"}, parseCSVFilter(" a , , b "))
}

func TestRolloutStatusDisabledRestartOnHashChange(t *testing.T) {
	assert.Equal(t, "disabled_restart", rolloutStatus(1, 0, "new", "old", "1.0.0", "1.0.0"))
}

func TestDisplayVersionUsesDefault(t *testing.T) {
	assert.Equal(t, data.DefaultAllocationVersion, displayVersion(""))
	assert.Equal(t, "2.0.0", displayVersion("2.0.0"))
}
