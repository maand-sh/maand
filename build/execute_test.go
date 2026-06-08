// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"maand/bucket"
	"maand/data"
	"maand/initialize"
)

func setupInitializedBuildBucket(t *testing.T) {
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
}

func writeMinimalBuildWorkspace(t *testing.T) {
	t.Helper()
	workersJSON := `[{"host":"10.0.0.1","labels":["web"],"memory":"1024","cpu":"2000","position":0}]`
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(workersJSON), 0o644))

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"version": "1.0.0",
		"selectors": ["web"],
		"resources": {
			"memory": {"min": "128", "max": "256"},
			"cpu": {"min": "100", "max": "200"}
		}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))
}

func TestExecute_syncsMinimalWorkspace(t *testing.T) {
	setupInitializedBuildBucket(t)
	writeMinimalBuildWorkspace(t)

	require.NoError(t, Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var workerCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM worker WHERE worker_ip = '10.0.0.1'`).Scan(&workerCount))
	assert.Equal(t, 1, workerCount)

	var allocCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM allocations WHERE job = 'api'`).Scan(&allocCount))
	assert.Equal(t, 1, allocCount)
}

func TestExecute_purgeJobCommandKVOption(t *testing.T) {
	setupInitializedBuildBucket(t)
	writeMinimalBuildWorkspace(t)

	require.NoError(t, Execute(Options{PurgeJobCommandKV: true}))
}

func TestExecute_rejectsInvalidDemand(t *testing.T) {
	setupInitializedBuildBucket(t)
	writeMinimalBuildWorkspace(t)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "consumer")
	require.NoError(t, os.MkdirAll(path.Join(jobPath, "_modules"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "_modules", "command_dep.py"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"version": "1.0.0",
		"selectors": ["web"],
		"commands": {
			"command_dep": {
				"executed_on": ["cli"],
				"demands": {"job": "api", "command": "missing_command"}
			}
		}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	err := Execute()
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidJobCommandDemand)
}

func TestExecute_fullWorkspacePipeline(t *testing.T) {
	setupInitializedBuildBucket(t)

	workersJSON := `[
		{"host":"10.0.0.1","labels":["web"],"memory":"1024","cpu":"2000","position":0},
		{"host":"10.0.0.2","labels":["web"],"memory":"1024","cpu":"2000","position":1}
	]`
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(workersJSON), 0o644))

	dbJobPath := path.Join(bucket.WorkspaceLocation, "jobs", "db")
	require.NoError(t, os.MkdirAll(path.Join(dbJobPath, "_modules"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(dbJobPath, "_modules", "command_schema.py"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(dbJobPath, "manifest.json"), []byte(`{
		"version": "2.0.0",
		"selectors": ["web"],
		"commands": {"command_schema": {"executed_on": ["cli"]}}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(dbJobPath, "Makefile"), []byte(""), 0o644))

	apiJobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(apiJobPath, "_modules"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(apiJobPath, "vars.toml"), []byte(`cluster = "primary"`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(apiJobPath, "_modules", "command_migrate.py"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(apiJobPath, "manifest.json"), []byte(`{
		"version": "1.0.0",
		"selectors": ["web"],
		"resources": {
			"memory": {"min": "128", "max": "256"},
			"cpu": {"min": "100", "max": "200"},
			"ports": {"http_port": {}}
		},
		"certs": {
			"tls": {
				"pkcs8": true,
				"one": false,
				"subject": {"common_name": "api.local"}
			}
		},
		"commands": {
			"command_migrate": {
				"executed_on": ["cli"],
				"demands": {
					"job": "db",
					"command": "command_schema",
					"config": {"min_version": "2.0.0"}
				}
			}
		}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(apiJobPath, "Makefile"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(`[api]
memory = "192 mb"
`), 0o644))

	require.NoError(t, Execute(Options{PurgeJobCommandKV: true}))
	require.NoError(t, Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var allocCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM allocations WHERE removed = 0`).Scan(&allocCount))
	assert.Equal(t, 4, allocCount)

	var dbSeq, apiSeq int
	require.NoError(t, db.QueryRow(`SELECT max(deployment_seq) FROM allocations WHERE job = 'db'`).Scan(&dbSeq))
	require.NoError(t, db.QueryRow(`SELECT max(deployment_seq) FROM allocations WHERE job = 'api'`).Scan(&apiSeq))
	assert.Less(t, dbSeq, apiSeq)
}

func TestExecute_runsPostBuildHook(t *testing.T) {
	setupInitializedBuildBucket(t)
	writeMinimalBuildWorkspace(t)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobPath, "_modules"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "_modules", "command_post_build.py"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"version": "1.0.0",
		"selectors": ["web"],
		"resources": {
			"memory": {"min": "128", "max": "256"},
			"cpu": {"min": "100", "max": "200"}
		},
		"commands": {
			"command_post_build": {"executed_on": ["post_build"]}
		}
	}`), 0o644))

	require.NoError(t, Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var commandCount int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM job_commands WHERE job = 'api' AND executed_on = 'post_build'`,
	).Scan(&commandCount))
	assert.Equal(t, 1, commandCount)
}

func TestExecute_notInitialized(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	err := Execute()
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrNotInitialized)
}
