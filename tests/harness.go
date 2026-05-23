// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"database/sql"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/data"
	"maand/initialize"

	"github.com/stretchr/testify/require"
)

// resetBucket removes the test project directory.
func resetBucket(t *testing.T) {
	t.Helper()
	require.NoError(t, os.RemoveAll(bucket.Location))
}

// initFreshBucket resets the tree and runs first-time maand init.
func initFreshBucket(t *testing.T) {
	t.Helper()
	resetBucket(t)
	require.NoError(t, initialize.Execute())
}

// upgradeBucket runs maand init again on an existing database (schema upgrade path).
func upgradeBucket(t *testing.T) {
	t.Helper()
	require.NoError(t, initialize.Execute())
}

func writeWorkersJSON(t *testing.T, content string) {
	t.Helper()
	path := path.Join(bucket.WorkspaceLocation, "workers.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func writeMinimalJob(t *testing.T, jobName, manifest string) {
	t.Helper()
	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", jobName)
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_modules"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
}

func mustGetBucketID(t *testing.T) string {
	t.Helper()
	var bucketID string
	MustQueryRow(t, `SELECT bucket_id FROM bucket LIMIT 1`, &bucketID)
	require.NotEmpty(t, bucketID)
	return bucketID
}

func mustGetSchemaVersion(t *testing.T) int {
	t.Helper()
	var version int
	MustQueryRow(t, `SELECT version FROM schema_version WHERE id = 1`, &version)
	return version
}

func mustTableExists(t *testing.T, tableName string) bool {
	t.Helper()
	count := MustQueryCount(t,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		tableName,
	)
	return count == 1
}

func mustViewExists(t *testing.T, viewName string) bool {
	t.Helper()
	count := MustQueryCount(t,
		`SELECT count(*) FROM sqlite_master WHERE type = 'view' AND name = ?`,
		viewName,
	)
	return count == 1
}

func requireLatestSchema(t *testing.T) {
	t.Helper()
	require.Equal(t, data.LatestSchemaVersion, mustGetSchemaVersion(t))
}

func runBuild(t *testing.T) {
	t.Helper()
	require.NoError(t, build.Execute())
}

func scanWorkerPositions(t *testing.T) map[string]int {
	t.Helper()
	workers := make(map[string]int)
	ScanQueryRows(t, `SELECT worker_ip, position FROM worker`, func(rows *sql.Rows) error {
		for rows.Next() {
			var workerIP string
			var position int
			if err := rows.Scan(&workerIP, &position); err != nil {
				return err
			}
			workers[workerIP] = position
		}
		return rows.Err()
	})
	return workers
}
