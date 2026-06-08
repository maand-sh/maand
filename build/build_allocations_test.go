// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"maand/bucket"
	"maand/data"
	"maand/workspace"

	_ "github.com/mattn/go-sqlite3"
)

func openBuildAllocationsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, data.MigrateSchema(tx))
	require.NoError(t, tx.Commit())
	return db
}

func seedBuildAllocationFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '1024', '2000', 0),
		       ('w2', '10.0.0.2', '1024', '2000', 1);
		INSERT INTO worker_labels (worker_id, label) VALUES ('w1', 'web'), ('w2', 'db');
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-web', 'web', '1.0.0', '0', '0', '0', '0', '0', '0', 1, ''),
		          ('job-db', 'db', '2.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO job_selectors (job_id, selector) VALUES ('job-web', 'web'), ('job-db', 'db');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('old-web', '10.0.0.1', 'web', 0, 0, 0, '0.0.0'),
		       ('old-db', '10.0.0.1', 'db', 0, 0, 0, '0.0.0');
		INSERT INTO hash (namespace, key, current_hash, previous_hash, current_version)
		VALUES ('web_allocation', 'old-web', 'cur', 'cur', '0.0.0');
	`)
	require.NoError(t, err)
}

func TestBuildAllocationsMatchesLabelsAndRemovesStaleJobs(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	seedBuildAllocationFixture(t, db)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, BuildAllocations(tx, workspace.Default()))
	require.NoError(t, tx.Commit())

	var webRemoved, dbRemoved int
	require.NoError(t, db.QueryRow(
		`SELECT removed FROM allocations WHERE worker_ip = '10.0.0.1' AND job = 'web'`,
	).Scan(&webRemoved))
	require.NoError(t, db.QueryRow(
		`SELECT removed FROM allocations WHERE worker_ip = '10.0.0.1' AND job = 'db'`,
	).Scan(&dbRemoved))
	assert.Equal(t, 0, webRemoved)
	assert.Equal(t, 1, dbRemoved)

	var webCount int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM allocations WHERE worker_ip = '10.0.0.2' AND job = 'db' AND removed = 0`,
	).Scan(&webCount))
	assert.Equal(t, 1, webCount)

	var newVersion string
	require.NoError(t, db.QueryRow(
		`SELECT new_version FROM allocations WHERE worker_ip = '10.0.0.1' AND job = 'web'`,
	).Scan(&newVersion))
	assert.Equal(t, "1.0.0", newVersion)
}

func TestBuildAllocationsAppliesDisabledConfigAndReenable(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	disabled := workspace.DisabledAllocations{
		Jobs: map[string]struct {
			Allocations []string `json:"allocations"`
		}{
			"web": {Allocations: []string{"10.0.0.1"}},
		},
	}
	disabledJSON, err := json.Marshal(disabled)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "disabled.json"), disabledJSON, 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	seedBuildAllocationFixture(t, db)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, BuildAllocations(tx, workspace.Default()))
	require.NoError(t, tx.Commit())

	var disabledFlag int
	require.NoError(t, db.QueryRow(
		`SELECT disabled FROM allocations WHERE worker_ip = '10.0.0.1' AND job = 'web'`,
	).Scan(&disabledFlag))
	assert.Equal(t, 1, disabledFlag)

	require.NoError(t, os.Remove(path.Join(bucket.WorkspaceLocation, "disabled.json")))

	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, BuildAllocations(tx, workspace.Default()))
	require.NoError(t, tx.Commit())

	require.NoError(t, db.QueryRow(
		`SELECT disabled FROM allocations WHERE worker_ip = '10.0.0.1' AND job = 'web'`,
	).Scan(&disabledFlag))
	assert.Equal(t, 0, disabledFlag)

	var previousHash sql.NullString
	require.NoError(t, db.QueryRow(
		`SELECT previous_hash FROM hash WHERE namespace = 'web_allocation' AND key = 'old-web'`,
	).Scan(&previousHash))
	assert.False(t, previousHash.Valid)
}

func TestBuildAllocations_disablesWorkerFromConfig(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	disabled := workspace.DisabledAllocations{Workers: []string{"10.0.0.1"}}
	disabledJSON, err := json.Marshal(disabled)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "disabled.json"), disabledJSON, 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	seedBuildAllocationFixture(t, db)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, BuildAllocations(tx, workspace.Default()))
	require.NoError(t, tx.Commit())

	var disabledCount int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM allocations WHERE worker_ip = '10.0.0.1' AND disabled = 1`,
	).Scan(&disabledCount))
	assert.Equal(t, 2, disabledCount)
}

func TestBuildAllocations_disablesEntireJobFromConfig(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	disabled := workspace.DisabledAllocations{
		Jobs: map[string]struct {
			Allocations []string `json:"allocations"`
		}{
			"web": {},
		},
	}
	disabledJSON, err := json.Marshal(disabled)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "disabled.json"), disabledJSON, 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	seedBuildAllocationFixture(t, db)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, BuildAllocations(tx, workspace.Default()))
	require.NoError(t, tx.Commit())

	var disabledCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM allocations WHERE job = 'web' AND disabled = 1`).Scan(&disabledCount))
	assert.Equal(t, 1, disabledCount)
}

func TestBuildAllocations_createsNewAllocation(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	_, err := db.Exec(`
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w3', '10.0.0.3', '1024', '2000', 2);
		INSERT INTO worker_labels (worker_id, label) VALUES ('w3', 'web');
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-web', 'web', '2.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO job_selectors (job_id, selector) VALUES ('job-web', 'web');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, BuildAllocations(tx, workspace.Default()))
	require.NoError(t, tx.Commit())

	var allocCount int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM allocations WHERE worker_ip = '10.0.0.3' AND job = 'web' AND removed = 0`,
	).Scan(&allocCount))
	assert.Equal(t, 1, allocCount)

	var newVersion string
	require.NoError(t, db.QueryRow(
		`SELECT new_version FROM allocations WHERE worker_ip = '10.0.0.3' AND job = 'web'`,
	).Scan(&newVersion))
	assert.Equal(t, "2.0.0", newVersion)
}

func TestLoadDisabledAllocations(t *testing.T) {
	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'web', 1, 0, 0, '0.0.0'),
		       ('a2', '10.0.0.2', 'web', 0, 1, 0, '0.0.0')`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	disabled, err := loadDisabledAllocations(tx)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())

	assert.Contains(t, disabled, "10.0.0.1|web")
	assert.NotContains(t, disabled, "10.0.0.2|web")
}

func TestMarkReenabledAllocations(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

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
	`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('alloc-1', '10.0.0.1', 'app', 0, 0, 0, '1.0.0')`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO hash (namespace, key, current_hash, previous_hash, current_version)
		VALUES ('app_allocation', 'alloc-1', 'cur', 'cur', '1.0.0')`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, markReenabledAllocations(tx, map[string]struct{}{"10.0.0.1|app": {}}))
	require.NoError(t, tx.Commit())

	var previousHash sql.NullString
	err = db.QueryRow(
		`SELECT previous_hash FROM hash WHERE namespace = 'app_allocation' AND key = 'alloc-1'`,
	).Scan(&previousHash)
	require.NoError(t, err)
	assert.False(t, previousHash.Valid)
}
