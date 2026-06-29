package deploy

import (
	"database/sql"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/kv"
	"maand/workspace"

	"github.com/stretchr/testify/require"
)

type deployTestEnv struct {
	root     string
	db       *sql.DB
	bucketID string
}

func setupDeployTestEnv(t *testing.T) *deployTestEnv {
	t.Helper()

	root := t.TempDir()
	oldLocation := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = oldLocation
		bucket.UpdatePath()
		ClearTestHooks()
	})

	for _, dir := range []string{
		path.Join(root, "data"),
		bucket.TempLocation,
		bucket.WorkspaceLocation,
		bucket.SecretLocation,
	} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.conf"), []byte(`ssh_user="agent"`), 0o644))

	db, err := sql.Open("sqlite3", data.DatabasePath())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, data.MigrateSchema(tx))
	bucketID := "test-bucket"
	require.NoError(t, data.InsertBucketRecord(tx, bucketID))
	require.NoError(t, data.SetBucketUpdateSeq(tx, 0))
	seedDeployKV(t, tx)
	require.NoError(t, tx.Commit())

	return &deployTestEnv{root: root, db: db, bucketID: bucketID}
}

func (e *deployTestEnv) begin(t *testing.T) *sql.Tx {
	t.Helper()
	tx, err := e.db.Begin()
	require.NoError(t, err)
	return tx
}

func seedDeployKV(t *testing.T, tx *sql.Tx) {
	t.Helper()
	_, err := tx.Exec(
		`INSERT INTO key_value (namespace, key, value, version, ttl, created_date, deleted)
		 VALUES ('maand/worker', 'certs/ca.crt', 'test-ca-cert', 1, '0', '0', 0)`,
	)
	require.NoError(t, err)
}

func (e *deployTestEnv) ensureWorker(t *testing.T, tx *sql.Tx, workerIP string, position int) {
	t.Helper()
	var count int
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM worker WHERE worker_ip = ?`, workerIP).Scan(&count))
	if count == 0 {
		e.insertWorker(t, tx, workerIP, position)
	}
}

func (e *deployTestEnv) insertWorker(t *testing.T, tx *sql.Tx, workerIP string, position int) string {
	t.Helper()
	workerID := "worker-" + workerIP
	_, err := tx.Exec(
		`INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		 VALUES (?, ?, '1024', '1000', ?)`,
		workerID, workerIP, position,
	)
	require.NoError(t, err)
	return workerID
}

func (e *deployTestEnv) insertJob(t *testing.T, tx *sql.Tx, jobName string, deploymentSeq, updateParallel int) string {
	t.Helper()
	jobID := "job-" + jobName
	_, err := tx.Exec(
		`INSERT INTO job (job_id, name, version, min_memory_mb, max_memory_mb, current_memory_mb,
		 min_cpu_mhz, max_cpu_mhz, current_cpu_mhz, max_concurrent_upgrades)
		 VALUES (?, ?, '1', '128', '256', '128', '100', '200', '100', ?)`,
		jobID, jobName, updateParallel,
	)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO job_selectors (job_id, selector) VALUES (?, 'worker')`, jobID)
	require.NoError(t, err)
	return jobID
}

func (e *deployTestEnv) insertAllocation(
	t *testing.T, tx *sql.Tx, allocID, workerIP, job string, deploymentSeq, disabled, removed int,
) {
	t.Helper()
	_, err := tx.Exec(
		`INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		allocID, workerIP, job, disabled, removed, deploymentSeq,
	)
	require.NoError(t, err)
}

func (e *deployTestEnv) insertJobFile(t *testing.T, tx *sql.Tx, jobID, filePath, content string, isDir bool) {
	t.Helper()
	isDirInt := 0
	if isDir {
		isDirInt = 1
	}
	_, err := tx.Exec(
		`INSERT INTO job_files (job_id, path, content, isdir) VALUES (?, ?, ?, ?)`,
		jobID, filePath, content, isDirInt,
	)
	require.NoError(t, err)
}

func (e *deployTestEnv) seedMakefileJob(t *testing.T, tx *sql.Tx, jobName, workerIP string, deploymentSeq int) {
	t.Helper()
	e.ensureWorker(t, tx, workerIP, 0)
	jobID := e.insertJob(t, tx, jobName, deploymentSeq, 1)
	allocID := "alloc-" + jobName + "-" + workerIP
	e.insertAllocation(t, tx, allocID, workerIP, jobName, deploymentSeq, 0, 0)
	e.insertJobFile(t, tx, jobID, path.Join(jobName, "data"), "", true)
	e.insertJobFile(t, tx, jobID, path.Join(jobName, "logs"), "", true)
	e.insertJobFile(t, tx, jobID, path.Join(jobName, "bin"), "", true)
	e.insertJobFile(t, tx, jobID, path.Join(jobName, "Makefile"), makefileContent(), false)
}

func (e *deployTestEnv) setRestartPolicy(t *testing.T, tx *sql.Tx, jobName, policy string) {
	t.Helper()
	_, err := tx.Exec(`UPDATE job SET restart_policy = ? WHERE name = ?`, policy, jobName)
	require.NoError(t, err)
}

func (e *deployTestEnv) setRestartGlobs(t *testing.T, tx *sql.Tx, jobName string, globs []string) {
	t.Helper()
	encoded, err := workspace.EncodeRestartGlobs(globs)
	require.NoError(t, err)
	_, err = tx.Exec(`UPDATE job SET restart_globs = ? WHERE name = ?`, encoded, jobName)
	require.NoError(t, err)
}

func makefileContent() string {
	return `.PHONY: start stop restart reload
dir:
	mkdir -p ./data ./logs ./bin
start: dir
	@echo $$(( $$(cat ./data/start 2>/dev/null || echo 0) + 1 )) > ./data/start
stop:
	mkdir -p ./data
	@echo $$(( $$(cat ./data/stop 2>/dev/null || echo 0) + 1 )) > ./data/stop
restart:
	mkdir -p ./data
	@echo $$(( $$(cat ./data/restart 2>/dev/null || echo 0) + 1 )) > ./data/restart
reload:
	mkdir -p ./data
	@echo $$(( $$(cat ./data/reload 2>/dev/null || echo 0) + 1 )) > ./data/reload
`
}

func (e *deployTestEnv) setAllocationHash(t *testing.T, tx *sql.Tx, job, allocID, current, previous string) {
	t.Helper()
	namespace := job + "_allocation"
	var prev any
	if previous != "" {
		prev = previous
	}
	_, err := tx.Exec(
		`INSERT INTO hash (namespace, key, current_hash, previous_hash) VALUES (?, ?, ?, ?)`,
		namespace, allocID, current, prev,
	)
	require.NoError(t, err)
}

func (e *deployTestEnv) allocationHashPromoted(t *testing.T, tx *sql.Tx, job, allocID string) bool {
	t.Helper()
	namespace := job + "_allocation"
	var current, previous sql.NullString
	err := tx.QueryRow(
		`SELECT current_hash, previous_hash FROM hash WHERE namespace = ? AND key = ?`,
		namespace, allocID,
	).Scan(&current, &previous)
	require.NoError(t, err)
	return current.Valid && previous.Valid && current.String == previous.String && current.String != ""
}

func installNoopDeployHooks(t *testing.T, bucketID string) *CommandRecorder {
	t.Helper()
	rec := &CommandRecorder{BucketID: bucketID}
	SetTestHooks(&TestHooks{
		WorkerCommand: rec.Record,
		Rsync: func(*bucket.Runtime, string, string, []string) error {
			return nil
		},
		SetupRuntime: func(string, bucket.RunContext) (*bucket.Runtime, error) {
			return nil, nil
		},
		CheckWorkerPrerequisites: func(*bucket.Runtime, []string) error {
			return nil
		},
	})
	t.Cleanup(ClearTestHooks)
	return rec
}

func initKV(t *testing.T, tx *sql.Tx) {
	t.Helper()
	require.NoError(t, kv.Initialize(tx))
}
