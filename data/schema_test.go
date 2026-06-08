// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"testing"

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
}

func TestApplySchemaMigrationUnsupportedVersion(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:unsupportedschema?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	err = applySchemaMigration(tx, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported schema version")
	require.NoError(t, tx.Rollback())
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
			update_parallel_count INT, health_check TEXT,
			PRIMARY KEY(name)
		);
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, migrateToV1(tx))
	require.NoError(t, tx.Commit())

	assert.Equal(t, 1, columnExists(t, db, "schema_version", "version"))
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
