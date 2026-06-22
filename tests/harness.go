// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"database/sql"
	"encoding/json"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/data"
	"maand/initialize"
	"maand/workspace"

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
	syncWorkerLabelsFromJobs(t)
}

// syncWorkerLabelsFromJobs adds the job name to workers when a job omits manifest selectors.
func syncWorkerLabelsFromJobs(tb testing.TB) {
	tb.Helper()
	jobsDir := path.Join(bucket.WorkspaceLocation, "jobs")
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		return
	}
	jobNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		jobName := entry.Name()
		manifestPath := path.Join(jobsDir, jobName, "manifest.json")
		raw, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest workspace.Manifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			continue
		}
		if shouldSyncJobNameLabel(manifest) {
			jobNames = append(jobNames, jobName)
		}
	}
	if len(jobNames) == 0 {
		return
	}

	workersPath := path.Join(bucket.WorkspaceLocation, "workers.json")
	raw, err := os.ReadFile(workersPath)
	if err != nil {
		return
	}

	var workers []map[string]any
	if err := json.Unmarshal(raw, &workers); err != nil {
		return
	}

	for i := range workers {
		labels := labelStrings(workers[i]["labels"])
		for _, jobName := range jobNames {
			if !containsString(labels, jobName) {
				labels = append(labels, jobName)
			}
		}
		workers[i]["labels"] = labels
	}

	updated, err := json.Marshal(workers)
	require.NoError(tb, err)
	require.NoError(tb, os.WriteFile(workersPath, updated, 0o644))
}

func shouldSyncJobNameLabel(manifest workspace.Manifest) bool {
	return len(manifest.Selectors) == 0
}

func labelStrings(raw any) []string {
	switch v := raw.(type) {
	case nil:
		return nil
	case []any:
		labels := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				labels = append(labels, s)
			}
		}
		return labels
	case []string:
		return append([]string(nil), v...)
	default:
		return nil
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func executeBuildErr(tb testing.TB, opts ...build.Options) error {
	tb.Helper()
	syncWorkerLabelsFromJobs(tb)
	if len(opts) > 0 {
		return build.Execute(opts[0])
	}
	return build.Execute()
}

func executeBuild(tb testing.TB, opts ...build.Options) {
	tb.Helper()
	require.NoError(tb, executeBuildErr(tb, opts...))
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
	executeBuild(t)
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
