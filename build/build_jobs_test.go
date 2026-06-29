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
	"maand/workspace"
)

func TestBuildJobsFromWorkspaceManifest(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"version": "1.0.0",
		"selectors": ["web"],
		"resources": {
			"memory": {"min": "128", "max": "256"},
			"cpu": {"min": "100", "max": "200"},
			"ports": {"http_port": {}}
		}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	removedJobs, err := BuildJobs(tx, workspace.Default())
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	assert.Empty(t, removedJobs)

	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM job WHERE name = 'api'`).Scan(&count))
	assert.Equal(t, 1, count)

	tx, err = db.Begin()
	require.NoError(t, err)
	version, err := data.GetJobVersion(tx, "api")
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	assert.Equal(t, "1.0.0", version)
}

func TestBuildJobs_storesPlacementSelectors(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["web"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.NoError(t, err)

	selectors, err := data.GetJobSelectors(tx, "api")
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())

	assert.ElementsMatch(t, []string{"web"}, selectors)
}

func TestBuildAllocations_placesJobByNameSelectorOnly(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	workersJSON := `[{"host":"10.0.0.1","labels":["prometheus"],"memory":"1024","cpu":"2000","position":0}]`
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(workersJSON), 0o644))

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "prometheus")
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildWorkers(tx, workspace.Default())
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.NoError(t, err)
	require.NoError(t, BuildAllocations(tx, workspace.Default()))
	require.NoError(t, tx.Commit())

	var allocCount int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM allocations WHERE job = 'prometheus' AND removed = 0`,
	).Scan(&allocCount))
	assert.Equal(t, 1, allocCount)
}

func TestBuildWorkers_rejectsDuplicateLabels(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	workersJSON := `[{"host":"10.0.0.1","labels":["web","web"],"memory":"1024","cpu":"2000","position":0}]`
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(workersJSON), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildWorkers(tx, workspace.Default())
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidWorkerJSON)
	require.NoError(t, tx.Rollback())
}

func TestBuildWorkersSyncsTagsAndRemovesStaleWorker(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	workersJSON := `[{"host":"10.0.0.1","labels":["web"],"tags":{"rack":"a1"},"memory":"1024","cpu":"2000","position":0}]`
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(workersJSON), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	_, err := db.Exec(`
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w-old', '10.0.0.99', '1024', '2000', 0);
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	removedWorkers, err := BuildWorkers(tx, workspace.Default())
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	assert.Equal(t, []string{"10.0.0.99"}, removedWorkers)

	var tagValue string
	require.NoError(t, db.QueryRow(
		`SELECT value FROM worker_tags WHERE worker_id = (SELECT worker_id FROM worker WHERE worker_ip = '10.0.0.1') AND key = 'rack'`,
	).Scan(&tagValue))
	assert.Equal(t, "a1", tagValue)
}

func TestBuildWorkersFromWorkspace(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	workersJSON := `[{"host":"10.0.0.1","labels":["web"],"memory":"1024","cpu":"2000","position":0}]`
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(workersJSON), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	removedWorkers, err := BuildWorkers(tx, workspace.Default())
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	assert.Empty(t, removedWorkers)

	var workerCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM worker WHERE worker_ip = '10.0.0.1'`).Scan(&workerCount))
	assert.Equal(t, 1, workerCount)

	var labelCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM worker_labels WHERE label = 'web'`).Scan(&labelCount))
	assert.GreaterOrEqual(t, labelCount, 1)
}

func TestSortedJobNames(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, sortedJobNames([]string{"c", "a", "b"}))
}

func TestBuildJobsRemovesStaleJobFromDatabase(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["web"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	_, err := db.Exec(`
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			max_concurrent_upgrades, health_check
		) VALUES ('job-legacy', 'legacy', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	removed, err := BuildJobs(tx, workspace.Default())
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	assert.Equal(t, []string{"legacy"}, removed)

	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM job WHERE name = 'legacy'`).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestBuildJobs_syncsFixedPort(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"selectors": ["web"],
		"resources": {
			"ports": {"metrics_port": 31000}
		}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	var port int
	require.NoError(t, db.QueryRow(`SELECT port FROM job_ports WHERE name = 'metrics_port'`).Scan(&port))
	assert.Equal(t, 31000, port)
}

func TestBuildJobs_walksWorkspaceFiles(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobPath, "config"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "config", "app.conf"), []byte("setting=1"), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["web"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	var content string
	require.NoError(t, db.QueryRow(
		`SELECT content FROM job_files WHERE path = 'api/config/app.conf'`,
	).Scan(&content))
	assert.Equal(t, "setting=1", content)
}

func TestBuildJobs_acceptsAllocationHookEvents(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobPath, "_modules"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "_modules", "command_rollout.py"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"selectors": ["web"],
		"commands": {"command_rollout": {"executed_on": ["after_allocation_started", "after_allocation_stopped"]}}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	var count int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM job_commands WHERE job = 'api' AND executed_on IN ('after_allocation_started', 'after_allocation_stopped')`,
	).Scan(&count))
	assert.Equal(t, 2, count)
}

func TestBuildJobs_rejectsInvalidCommandName(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobPath, "_modules"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "_modules", "bad.py"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"selectors": ["web"],
		"commands": {"bad": {"executed_on": ["cli"]}}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidJobCommandConfiguration)
	require.NoError(t, tx.Rollback())
}

func TestBuildJobs_rejectsUnsupportedMemoryRequest(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"selectors": ["web"],
		"resources": {"memory": {"min": "128", "max": "256"}}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(`[api]
memory = "512 mb"
`), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrUnsupportedResourceConfiguration)
	require.NoError(t, tx.Rollback())
}

func TestBuildJobs_rejectsInvalidCPU(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"selectors": ["web"],
		"resources": {
			"cpu": {"min": "500", "max": "100"}
		}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidManifest)
	require.NoError(t, tx.Rollback())
}

func TestBuildJobs_rejectsReservedDataDirectory(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobPath, "data"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["web"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidJob)
	require.NoError(t, tx.Rollback())
}

func TestBuildJobsSyncsHealthCheckAndCommands(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobPath, "_modules"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "_modules", "command_status.py"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"version": "1.0.0",
		"selectors": ["web"],
		"health_check": {
			"checks": [{"type": "http", "port": "http_port", "path": "/health"}]
		},
		"resources": {
			"memory": {"min": "128", "max": "256"},
			"ports": {"http_port": {}}
		},
		"commands": {
			"command_status": {
				"executed_on": ["cli"],
				"demands": {"job": "", "command": ""}
			}
		}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(`[api]
memory = "192 mb"
`), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	var healthCheck string
	require.NoError(t, db.QueryRow(`SELECT health_check FROM job WHERE name = 'api'`).Scan(&healthCheck))
	assert.Contains(t, healthCheck, "http")

	var commandCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM job_commands WHERE job = 'api'`).Scan(&commandCount))
	assert.Equal(t, 1, commandCount)

	var currentMemory string
	require.NoError(t, db.QueryRow(`SELECT current_memory_mb FROM job WHERE name = 'api'`).Scan(&currentMemory))
	assert.Equal(t, "192", currentMemory)

	var memorySource string
	require.NoError(t, db.QueryRow(`SELECT current_memory_source FROM job WHERE name = 'api'`).Scan(&memorySource))
	assert.Equal(t, "bucket.jobs.conf", memorySource)

	var cpuSource string
	require.NoError(t, db.QueryRow(`SELECT current_cpu_source FROM job WHERE name = 'api'`).Scan(&cpuSource))
	assert.Equal(t, "manifest", cpuSource)
}

func TestBuildJobsSyncsCertsFromManifest(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"selectors": ["web"],
		"certs": {
			"tls": {
				"pkcs8": true,
				"one": false,
				"subject": {"common_name": "api.local"}
			}
		}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	var certCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM job_certs WHERE name = 'tls'`).Scan(&certCount))
	assert.Equal(t, 1, certCount)
}

func TestBuildJobs_acceptsMakefileTemplate(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "echo_server")
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"selectors": ["worker"]
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile.tpl"), []byte("start:\n"), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	var jobCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM job WHERE name = 'echo_server'`).Scan(&jobCount))
	assert.Equal(t, 1, jobCount)
}

func TestBuildJobs_rejectsRestartGlobsWithoutReloadPolicy(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{
		"restart_policy": "always",
		"restart_globs": ["Makefile"],
		"selectors": ["worker"]
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = BuildJobs(tx, workspace.Default())
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidManifest)
	require.NoError(t, tx.Rollback())
}
