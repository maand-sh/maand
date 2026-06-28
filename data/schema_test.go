// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigrateSchemaUpgradesLegacyColumns(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:legacyschema?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	_, err = db.Exec(`
		CREATE TABLE allocations (
			alloc_id TEXT, worker_ip TEXT, job TEXT,
			disabled INT, removed INT, deployment_seq INT,
			PRIMARY KEY(worker_ip, job)
		);
		CREATE TABLE hash (
			namespace TEXT, key TEXT,
			current_hash TEXT, previous_hash TEXT,
			PRIMARY KEY(namespace, key)
		);
		CREATE TABLE job (
			job_id TEXT, name TEXT, version TEXT,
			min_memory_mb TEXT, max_memory_mb TEXT, current_memory_mb TEXT,
			min_cpu_mhz TEXT, max_cpu_mhz TEXT, current_cpu_mhz TEXT,
			update_parallel_count INT,
			PRIMARY KEY(name)
		);
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, MigrateSchema(tx))
	require.NoError(t, tx.Commit())

	assert.Equal(t, 1, columnExists(t, db, "allocations", "new_version"))
	assert.Equal(t, 1, columnExists(t, db, "hash", "current_version"))
	assert.Equal(t, 1, columnExists(t, db, "job", "health_check"))
	assert.Equal(t, 1, columnExists(t, db, "job", "deploy_parallel_count"))
	assert.Equal(t, 1, columnExists(t, db, "job", "restart_policy"))
	assert.Equal(t, 1, columnExists(t, db, "job", "restart_globs"))
	assert.Equal(t, 1, columnExists(t, db, "job", "current_memory_source"))
	assert.Equal(t, 1, columnExists(t, db, "job", "current_cpu_source"))
	assert.Equal(t, 1, columnExists(t, db, "hash", "current_files"))
	assert.Equal(t, 1, columnExists(t, db, "hash", "previous_files"))
	assert.Equal(t, 1, viewColumnExists(t, db, "cat_jobs", "current_memory_source"))
	assert.Equal(t, 1, viewColumnExists(t, db, "cat_jobs", "current_cpu_source"))
	assert.Equal(t, 1, viewExists(t, db, "cat_deployments"))
	assert.Equal(t, 0, viewExists(t, db, "cat_hashes"))
}

func viewColumnExists(t *testing.T, db *sql.DB, view, column string) int {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM pragma_table_info(?) WHERE name = ?`,
		view, column,
	).Scan(&count))
	return count
}

func withTestBucket(t *testing.T) func() {
	t.Helper()
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	return func() {
		bucket.Location = orig
	}
}

func TestCheckSchemaVersionMissingDatabase(t *testing.T) {
	defer withTestBucket(t)()
	err := CheckSchemaVersion()
	require.ErrorIs(t, err, bucket.ErrNotInitialized)
}

func TestCheckSchemaVersionRequiresUpgrade(t *testing.T) {
	defer withTestBucket(t)()
	require.NoError(t, os.MkdirAll(path.Join(bucket.Location, "data"), 0o755))

	diskDB, err := sql.Open("sqlite3", DatabasePath())
	require.NoError(t, err)
	_, err = diskDB.Exec(`
		CREATE TABLE allocations (
			alloc_id TEXT, worker_ip TEXT, job TEXT,
			disabled INT, removed INT, deployment_seq INT,
			PRIMARY KEY(worker_ip, job)
		);
	`)
	require.NoError(t, err)
	require.NoError(t, diskDB.Close())

	err = CheckSchemaVersion()
	require.ErrorIs(t, err, bucket.ErrSchemaUpgradeRequired)
	assert.Contains(t, err.Error(), "run maand init")
}

func TestCheckSchemaVersionRequiresUpgradeForLegacyVersion(t *testing.T) {
	defer withTestBucket(t)()
	db := openDatabaseAtSchemaVersion(t, 4)
	defer func() { _ = db.Close() }()

	err := CheckSchemaVersion()
	require.ErrorIs(t, err, bucket.ErrSchemaUpgradeRequired)
	assert.Contains(t, err.Error(), "binary expects 1")
}

func TestMigrateSchemaRenumbersLegacyVersion(t *testing.T) {
	defer withTestBucket(t)()
	db := openDatabaseAtSchemaVersion(t, 4)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, MigrateSchema(tx))
	require.NoError(t, tx.Commit())

	var version int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version WHERE id = 1`).Scan(&version))
	assert.Equal(t, LatestSchemaVersion, version)
	require.NoError(t, CheckSchemaVersion())
}

func TestMigrateToV1IsIdempotent(t *testing.T) {
	defer withTestBucket(t)()
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, migrateToV1(tx))
	require.NoError(t, migrateToV1(tx))
	require.NoError(t, tx.Commit())

	assert.Equal(t, 1, viewExists(t, db, "cat_deployments"))
	assert.Equal(t, 0, viewExists(t, db, "cat_hashes"))
}

func TestMigrateToV1AddsDeployParallelCount(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:migratev1deploy?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		CREATE TABLE allocations (
			alloc_id TEXT, worker_ip TEXT, job TEXT,
			disabled INT, removed INT, deployment_seq INT,
			PRIMARY KEY(worker_ip, job)
		);
		CREATE TABLE hash (
			namespace TEXT, key TEXT,
			current_hash TEXT, previous_hash TEXT,
			PRIMARY KEY(namespace, key)
		);
		CREATE TABLE job (
			job_id TEXT, name TEXT, version TEXT,
			min_memory_mb TEXT, max_memory_mb TEXT, current_memory_mb TEXT,
			min_cpu_mhz TEXT, max_cpu_mhz TEXT, current_cpu_mhz TEXT,
			update_parallel_count INT,
			PRIMARY KEY(name)
		);
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO job (job_id, name, version, update_parallel_count) VALUES ('job-api', 'api', '1.0.0', 1)`)
	require.NoError(t, err)
	require.NoError(t, migrateToV1(tx))
	require.NoError(t, tx.Commit())

	var deployParallel int
	err = db.QueryRow(`SELECT deploy_parallel_count FROM job WHERE name = 'api'`).Scan(&deployParallel)
	require.NoError(t, err)
	assert.Equal(t, 0, deployParallel)
}

func TestCatDeploymentsViewJoinsAllocationAndHash(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	var currentHash string
	err = db.QueryRow(
		`SELECT current_hash FROM cat_deployments WHERE alloc_id = 'alloc-1'`,
	).Scan(&currentHash)
	require.NoError(t, err)
	assert.Equal(t, "hash-a", currentHash)
}

func TestCheckSchemaVersionCurrent(t *testing.T) {
	defer withTestBucket(t)()
	require.NoError(t, os.MkdirAll(path.Join(bucket.Location, "data"), 0o755))

	db, err := OpenDatabase(false)
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, MigrateSchema(tx))
	require.NoError(t, tx.Commit())
	require.NoError(t, db.Close())

	require.NoError(t, CheckSchemaVersion())
}

func TestCheckSchemaVersionRequiresUpgradeForMissingColumn(t *testing.T) {
	defer withTestBucket(t)()
	require.NoError(t, os.MkdirAll(path.Join(bucket.Location, "data"), 0o755))

	db, err := OpenDatabase(false)
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, execStatements(tx, append(baseTableDDL(),
		`CREATE TABLE IF NOT EXISTS schema_version (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL
		)`,
		`INSERT INTO schema_version (id, version) VALUES (1, 1)`,
	)))
	_, err = tx.Exec(`ALTER TABLE job DROP COLUMN restart_policy`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, db.Close())

	err = CheckSchemaVersion()
	require.ErrorIs(t, err, bucket.ErrSchemaUpgradeRequired)
	assert.Contains(t, err.Error(), "restart_policy")
	assert.Contains(t, err.Error(), "run maand init")
}

func TestCheckSchemaVersionRequiresUpgradeForStaleCatalogView(t *testing.T) {
	defer withTestBucket(t)()
	require.NoError(t, os.MkdirAll(path.Join(bucket.Location, "data"), 0o755))

	db, err := OpenDatabase(false)
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, MigrateSchema(tx))
	_, err = tx.Exec(`DROP VIEW IF EXISTS cat_jobs`)
	require.NoError(t, err)
	_, err = tx.Exec(`CREATE VIEW cat_jobs (job_id, name, version, disabled, deployment_seq, selectors) AS
		SELECT job_id, name, version, 0, 0, '' FROM job`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, db.Close())

	err = CheckSchemaVersion()
	require.ErrorIs(t, err, bucket.ErrSchemaUpgradeRequired)
	assert.Contains(t, err.Error(), "cat_jobs")
	assert.Contains(t, err.Error(), "current_memory_mb")
	assert.Contains(t, err.Error(), "run maand init")
}

func TestCheckSchemaVersionTooNew(t *testing.T) {
	defer withTestBucket(t)()
	require.NoError(t, os.MkdirAll(path.Join(bucket.Location, "data"), 0o755))

	db, err := OpenDatabase(false)
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, execStatements(tx, append(baseTableDDL(),
		`CREATE TABLE IF NOT EXISTS schema_version (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL
		)`,
	)))
	require.NoError(t, writeSchemaVersion(tx, legacySchemaVersionMax+1))
	require.NoError(t, tx.Commit())
	require.NoError(t, db.Close())

	err = CheckSchemaVersion()
	require.ErrorIs(t, err, bucket.ErrSchemaTooNew)
}

func TestMigrateSchemaIsIdempotent(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, MigrateSchema(tx))
	require.NoError(t, tx.Commit())

	var version int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version WHERE id = 1`).Scan(&version))
	assert.Equal(t, LatestSchemaVersion, version)
}

func TestReadSchemaVersionWhenTableEmpty(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:emptyschemaversion?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE TABLE schema_version (id INTEGER PRIMARY KEY, version INTEGER NOT NULL)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	version, err := readSchemaVersion(tx)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	assert.Equal(t, 0, version)
}

func TestReadSchemaVersionOnFreshDatabase(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:freshschema?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	version, err := readSchemaVersion(tx)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	assert.Equal(t, 0, version)
}

func TestMigrateToV1AddsSchemaVersionTable(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:migratev1?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		CREATE TABLE allocations (
			alloc_id TEXT, worker_ip TEXT, job TEXT,
			disabled INT, removed INT, deployment_seq INT,
			PRIMARY KEY(worker_ip, job)
		);
		CREATE TABLE hash (
			namespace TEXT, key TEXT,
			current_hash TEXT, previous_hash TEXT,
			PRIMARY KEY(namespace, key)
		);
		CREATE TABLE job (
			job_id TEXT, name TEXT, version TEXT,
			min_memory_mb TEXT, max_memory_mb TEXT, current_memory_mb TEXT,
			min_cpu_mhz TEXT, max_cpu_mhz TEXT, current_cpu_mhz TEXT,
			update_parallel_count INT,
			PRIMARY KEY(name)
		);
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, migrateToV1(tx))
	require.NoError(t, writeSchemaVersion(tx, 1))
	require.NoError(t, tx.Commit())

	assert.Equal(t, 1, columnExists(t, db, "allocations", "new_version"))
	assert.Equal(t, 1, columnExists(t, db, "schema_version", "version"))
	assert.Equal(t, 1, viewExists(t, db, "cat_deployments"))
}

func TestMigrateToV1WithPreexistingColumns(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:migratev1skip?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		CREATE TABLE allocations (
			alloc_id TEXT, worker_ip TEXT, job TEXT,
			disabled INT, removed INT, deployment_seq INT, new_version TEXT,
			PRIMARY KEY(worker_ip, job)
		);
		CREATE TABLE hash (
			namespace TEXT, key TEXT,
			current_hash TEXT, previous_hash TEXT, current_version TEXT,
			PRIMARY KEY(namespace, key)
		);
		CREATE TABLE job (
			job_id TEXT, name TEXT, version TEXT,
			min_memory_mb TEXT, max_memory_mb TEXT, current_memory_mb TEXT,
			min_cpu_mhz TEXT, max_cpu_mhz TEXT, current_cpu_mhz TEXT,
			update_parallel_count INT, deploy_parallel_count INT NOT NULL DEFAULT 0, health_check TEXT,
			PRIMARY KEY(name)
		);
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, migrateToV1(tx))
	require.NoError(t, tx.Commit())

	assert.Equal(t, 1, columnExists(t, db, "schema_version", "version"))
	assert.Equal(t, 1, viewExists(t, db, "cat_deployments"))
}

func openDatabaseAtSchemaVersion(t *testing.T, targetVersion int) *sql.DB {
	t.Helper()
	require.NoError(t, os.MkdirAll(path.Join(bucket.Location, "data"), 0o755))

	db, err := OpenDatabase(false)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, execStatements(tx, baseTableDDL()))
	require.NoError(t, execStatements(tx, []string{
		`CREATE TABLE IF NOT EXISTS schema_version (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL
		)`,
	}))
	if targetVersion > 0 {
		require.NoError(t, writeSchemaVersion(tx, targetVersion))
	}
	require.NoError(t, tx.Commit())
	return db
}

func columnExists(t *testing.T, db *sql.DB, table, column string) int {
	t.Helper()
	var count int
	err := db.QueryRow(
		`SELECT count(*) FROM pragma_table_info(?) WHERE name = ?`,
		table, column,
	).Scan(&count)
	require.NoError(t, err)
	return count
}

func viewExists(t *testing.T, db *sql.DB, viewName string) int {
	t.Helper()
	var count int
	err := db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type = 'view' AND name = ?`,
		viewName,
	).Scan(&count)
	require.NoError(t, err)
	return count
}
