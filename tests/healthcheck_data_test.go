package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetJobHealthCheckFromDB(t *testing.T) {
	initFreshBucket(t)

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	const healthJSON = `{"checks":[{"type":"tcp","port":"api_port"}]}`
	_, err = tx.Exec(`
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('id1', 'api', '1', '0', '0', '0', '0', '0', '0', 1, ?)`,
		healthJSON,
	)
	require.NoError(t, err)

	spec, err := data.GetJobHealthCheck(tx, "api")
	require.NoError(t, err)
	require.NotNil(t, spec)
	require.Len(t, spec.Checks, 1)
	assert.Equal(t, "tcp", spec.Checks[0].Type)
	assert.Equal(t, "api_port", spec.Checks[0].Port)
}

func TestCopyJobCommandModuleFromDB(t *testing.T) {
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
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	const jobName = "app"
	const jobID = "job-id"
	_, err = tx.Exec(`
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count
		) VALUES (?, ?, '1', '0', '0', '0', '0', '0', '0', 1)`,
		jobID, jobName,
	)
	require.NoError(t, err)

	for _, row := range []struct {
		path    string
		content string
		isDir   bool
	}{
		{"app/_modules/command_health.py", `print("ok")`, false},
		{"app/Makefile", "start:\n", false},
	} {
		_, err = tx.Exec(
			`INSERT INTO job_files (job_id, path, content, isdir) VALUES (?, ?, ?, ?)`,
			jobID, row.path, []byte(row.content), row.isDir,
		)
		require.NoError(t, err)
	}

	outDir := t.TempDir()
	require.NoError(t, data.CopyJobCommandModule(tx, jobName, "command_health", outDir))

	script, err := os.ReadFile(path.Join(outDir, "app/_modules/command_health.py"))
	require.NoError(t, err)
	assert.Equal(t, `print("ok")`, string(script))
	_, err = os.Stat(path.Join(outDir, "app/Makefile"))
	assert.True(t, os.IsNotExist(err))
}
