// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func openMigratedTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, MigrateSchema(tx))
	require.NoError(t, tx.Commit())
	return db
}

func seedWorkerJobAllocation(t *testing.T, tx *sql.Tx) {
	t.Helper()
	_, err := tx.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 3);
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '1024', '2000', 0),
		       ('w2', '10.0.0.2', '1024', '2000', 1);
		INSERT INTO worker_labels (worker_id, label) VALUES ('w1', 'web'), ('w2', 'web');
		INSERT INTO worker_tags (worker_id, key, value) VALUES ('w1', 'role', 'primary');
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-api', 'api', '1.2.3', '128', '256', '128', '100', '200', '100', 1, '');
		INSERT INTO job_selectors (job_id, selector) VALUES ('job-api', 'web');
		INSERT INTO job_ports (job_id, name, port) VALUES ('job-api', 'http_port', 8080);
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('alloc-1', '10.0.0.1', 'api', 0, 0, 1, '2.0.0'),
		       ('alloc-2', '10.0.0.2', 'api', 1, 0, 1, '2.0.0');
		INSERT INTO hash (namespace, key, current_hash, previous_hash, current_version)
		VALUES ('api_allocation', 'alloc-1', 'hash-a', 'hash-a', '1.0.0'),
		       ('api_allocation', 'alloc-2', 'hash-b', 'hash-b', '2.0.0');
	`)
	require.NoError(t, err)
}
