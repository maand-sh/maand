// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/data"
	"maand/deploy"
	"maand/initialize"
	"maand/worker"

	"github.com/stretchr/testify/require"
)

const integrationJobName = "integapp"

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "go.mod not found")
		dir = parent
	}
}

func assetsDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("MAAND_INTEGRATION_ASSETS"); dir != "" {
		return dir
	}
	return filepath.Join(repoRoot(t), "assets")
}

func requireIntegrationAssets(t *testing.T) {
	t.Helper()
	root := assetsDir(t)
	for _, name := range []string{"workers.json", "worker.key", "maand.conf"} {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err != nil {
			t.Skipf("integration assets missing %s — see assets/README.md", path)
		}
	}
}

func resetIntegrationBucket(t *testing.T) {
	t.Helper()
	require.NoError(t, os.RemoveAll(bucket.Location))
}

func copyFile(t *testing.T, src, dst string, mode os.FileMode) {
	t.Helper()
	in, err := os.Open(src)
	require.NoError(t, err)
	defer func() { _ = in.Close() }()

	require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755))
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	require.NoError(t, err)
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	require.NoError(t, err)
	require.NoError(t, out.Sync())
}

func installAssets(t *testing.T) {
	t.Helper()
	root := assetsDir(t)
	copyFile(t, filepath.Join(root, "workers.json"), filepath.Join(bucket.WorkspaceLocation, "workers.json"), 0o644)
	copyFile(t, filepath.Join(root, "worker.key"), filepath.Join(bucket.SecretLocation, "worker.key"), 0o600)
	copyFile(t, filepath.Join(root, "maand.conf"), filepath.Join(bucket.Location, "maand.conf"), 0o644)
}

func writeIntegrationJob(t *testing.T) {
	t.Helper()
	jobDir := filepath.Join(bucket.WorkspaceLocation, "jobs", integrationJobName)
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "Makefile"), []byte(integrationMakefile()), 0o644))
}

func integrationMakefile() string {
	return `.PHONY: start stop restart status migrate
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
status:
	@cat ./data/start 2>/dev/null || echo 0
migrate:
	mkdir -p ./data
	@echo migrated > ./data/migrate
`
}

// setupIntegrationWorkspace initializes a fresh bucket from assets/ and creates the integration job (no build).
func setupIntegrationWorkspace(t *testing.T) {
	t.Helper()
	requireIntegrationAssets(t)
	resetIntegrationBucket(t)
	require.NoError(t, initialize.Execute())
	installAssets(t)
	writeIntegrationJob(t)
}

// setupIntegrationBucket initializes a fresh bucket from assets/, creates the integration job, and runs build.
func setupIntegrationBucket(t *testing.T) {
	t.Helper()
	setupIntegrationWorkspace(t)
	require.NoError(t, build.Execute())
}

// setupPortOnlyIntegrationBucket initializes a bucket with a single port job and runs build.
// It does not require integration assets (port stability is local-only).
func setupPortOnlyIntegrationBucket(t *testing.T) {
	t.Helper()
	resetIntegrationBucket(t)
	require.NoError(t, initialize.Execute())
	writePortOnlyIntegrationJob(t)
	require.NoError(t, build.Execute())
}

func writePortOnlyIntegrationJob(t *testing.T) {
	t.Helper()
	jobDir := filepath.Join(bucket.WorkspaceLocation, "jobs", integrationJobName)
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "manifest.json"), []byte(`{
		"resources": {"ports": {"integapp_port_http": {}}}
	}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "Makefile"), []byte("start:\n"), 0o644))
}

// setupFullIntegrationBucket is like setupIntegrationBucket but registers job commands and a port.
func setupFullIntegrationBucket(t *testing.T) {
	t.Helper()
	requireIntegrationAssets(t)
	resetIntegrationBucket(t)
	require.NoError(t, initialize.Execute())
	installAssets(t)
	writeFullIntegrationJob(t)
	require.NoError(t, build.Execute())
}

func workerIPs(t *testing.T) []string {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	ips, err := data.GetWorkers(tx, nil)
	require.NoError(t, err)
	require.NotEmpty(t, ips)
	return ips
}

func replaceWorkersJSON(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(bucket.WorkspaceLocation, "workers.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func countAllocations(t *testing.T, removed bool) int {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	flag := 0
	if removed {
		flag = 1
	}
	var count int
	require.NoError(t, tx.QueryRow(
		`SELECT count(*) FROM allocations WHERE job = ? AND removed = ?`,
		integrationJobName, flag,
	).Scan(&count))
	return count
}

func mustBucketID(t *testing.T) string {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	bucketID, err := data.GetBucketID(tx)
	require.NoError(t, err)
	require.NotEmpty(t, bucketID)
	return bucketID
}

func writeFullIntegrationJob(t *testing.T) {
	t.Helper()
	jobDir := filepath.Join(bucket.WorkspaceLocation, "jobs", integrationJobName)
	modulesDir := filepath.Join(jobDir, "_modules")
	require.NoError(t, os.MkdirAll(modulesDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["worker"],
		"resources": {"ports": {"integapp_port_http": {}}},
		"commands": {
			"command_health_check": {"executed_on": ["health_check"]},
			"command_cli": {"executed_on": ["cli"]},
			"command_cli_kv": {"executed_on": ["cli"]},
			"command_cli_secret": {"executed_on": ["cli"]},
			"command_pre_deploy": {"executed_on": ["pre_deploy"]},
			"command_post_deploy": {"executed_on": ["post_deploy"]}
		}
	}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "Makefile"), []byte(integrationMakefile()), 0o644))

	scripts := map[string]string{
		"command_health_check.py": `print("maand-health-check-ok")`,
		"command_cli.py":          `print("maand-cli-ok")`,
		"command_cli_kv.py": `import json
import os
import urllib.error
import urllib.request

def runtime_request(method, path, body=None):
    host = os.environ.get("JOB_COMMAND_API_HOST", "127.0.0.1")
    headers = {
        "X-ALLOCATION-ID": os.environ["ALLOCATION_ID"],
        "COMMAND": os.environ["COMMAND"],
        "EVENT": os.environ["EVENT"],
        "Content-Type": "application/json",
    }
    data = None
    if body is not None:
        data = json.dumps(body).encode()
    req = urllib.request.Request(
        f"http://{host}:8080{path}",
        data=data,
        headers=headers,
        method=method,
    )
    with urllib.request.urlopen(req) as resp:
        raw = resp.read().decode()
        if not raw.strip():
            return {}
        return json.loads(raw)

job = os.environ["JOB"]
namespace = f"vars/job/{job}"
runtime_request("PUT", "/kv", {"namespace": namespace, "key": "integ_key", "value": "integ_value"})
got = runtime_request("GET", "/kv", {"namespace": namespace, "key": "integ_key"})
if got.get("value") != "integ_value":
    raise SystemExit(1)
print("maand-kv-ok")`,
		"command_cli_secret.py": `import json
import os
import urllib.request

def runtime_request(method, path, body=None):
    host = os.environ.get("JOB_COMMAND_API_HOST", "127.0.0.1")
    headers = {
        "X-ALLOCATION-ID": os.environ["ALLOCATION_ID"],
        "COMMAND": os.environ["COMMAND"],
        "EVENT": os.environ["EVENT"],
        "Content-Type": "application/json",
    }
    data = None
    if body is not None:
        data = json.dumps(body).encode()
    req = urllib.request.Request(
        f"http://{host}:8080{path}",
        data=data,
        headers=headers,
        method=method,
    )
    with urllib.request.urlopen(req) as resp:
        raw = resp.read().decode()
        if not raw.strip():
            return {}
        return json.loads(raw)

job = os.environ["JOB"]
namespace = f"secrets/job/{job}"
runtime_request("PUT", "/kv/secret", {"namespace": namespace, "key": "integ_secret", "value": "secret-value"})
got = runtime_request("GET", "/kv", {"namespace": namespace, "key": "integ_secret"})
if got.get("value") != "secret-value":
    raise SystemExit(1)
print("maand-secret-ok")`,
		"command_pre_deploy.py":  `print("maand-pre-deploy")`,
		"command_post_deploy.py": `print("maand-post-deploy")`,
	}
	for name, body := range scripts {
		require.NoError(t, os.WriteFile(filepath.Join(modulesDir, name), []byte(body), 0o644))
	}
}

func countJobAllocationHashes(t *testing.T, job string) int {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	var count int
	namespace := fmt.Sprintf("%s_allocation", job)
	require.NoError(t, tx.QueryRow(
		`SELECT count(*) FROM hash WHERE namespace = ?`, namespace,
	).Scan(&count))
	return count
}

func jobAllocationHashesPromoted(t *testing.T, job string) bool {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	workers, err := data.GetActiveAllocations(tx, job)
	require.NoError(t, err)
	if len(workers) == 0 {
		return false
	}

	namespace := fmt.Sprintf("%s_allocation", job)
	for _, workerIP := range workers {
		allocID, err := data.GetAllocationID(tx, workerIP, job)
		require.NoError(t, err)

		var currentHash, previousHash sql.NullString
		err = tx.QueryRow(
			`SELECT current_hash, previous_hash FROM hash WHERE namespace = ? AND key = ?`,
			namespace, allocID,
		).Scan(&currentHash, &previousHash)
		if err != nil {
			return false
		}
		if !currentHash.Valid || currentHash.String == "" {
			return false
		}
		if !previousHash.Valid || previousHash.String != currentHash.String {
			return false
		}
	}
	return true
}

func writeRollingIntegrationJob(t *testing.T, updateParallel int) {
	t.Helper()
	jobDir := filepath.Join(bucket.WorkspaceLocation, "jobs", integrationJobName)
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	manifest := fmt.Sprintf(`{"selectors":["worker"],"update_parallel_count":%d}`, updateParallel)
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "manifest.json"), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "Makefile"), []byte(integrationMakefile()), 0o644))
}

func setupRollingIntegrationBucket(t *testing.T, updateParallel int) {
	t.Helper()
	requireIntegrationAssets(t)
	resetIntegrationBucket(t)
	require.NoError(t, initialize.Execute())
	installAssets(t)
	writeRollingIntegrationJob(t, updateParallel)
	require.NoError(t, build.Execute())
}

func jobUpdateParallelCount(t *testing.T, job string) int {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	count, err := data.GetUpdateParallelCount(tx, job)
	require.NoError(t, err)
	return count
}

func dryRunPlanForJob(t *testing.T, job string) deploy.JobPlan {
	t.Helper()
	result, err := deploy.DryRun([]string{job})
	require.NoError(t, err)
	for _, plan := range result.Jobs {
		if plan.Job == job {
			return plan
		}
	}
	t.Fatalf("dry-run plan for job %q not found", job)
	return deploy.JobPlan{}
}

func workerJobCounter(t *testing.T, workerIP, counter string) int {
	t.Helper()
	bucketID := mustBucketID(t)
	path := fmt.Sprintf(
		"/opt/worker/%s/jobs/%s/data/%s",
		bucketID, integrationJobName, counter,
	)
	raw := strings.TrimSpace(remoteShellOutput(t, workerIP, fmt.Sprintf("cat %s 2>/dev/null || echo 0", path)))
	value, err := strconv.Atoi(raw)
	require.NoError(t, err, "counter %s on %s: %q", counter, workerIP, raw)
	return value
}

func remoteShellOutput(t *testing.T, workerIP, command string) string {
	t.Helper()
	out, err := worker.RemoteShellOutput(workerIP, command)
	require.NoError(t, err, "ssh %s", workerIP)
	return out
}

func versionTrackingMakefile() string {
	return `.PHONY: start stop restart status migrate
dir:
	mkdir -p ./data ./logs ./bin
start: dir
	@echo "$(CURRENT_VERSION)" > ./data/current_version
	@echo "$(NEW_VERSION)" > ./data/new_version
	@echo $$(( $$(cat ./data/start 2>/dev/null || echo 0) + 1 )) > ./data/start
stop:
	mkdir -p ./data
	@echo $$(( $$(cat ./data/stop 2>/dev/null || echo 0) + 1 )) > ./data/stop
restart:
	mkdir -p ./data
	@echo "$(CURRENT_VERSION)" > ./data/current_version
	@echo "$(NEW_VERSION)" > ./data/new_version
	@echo $$(( $$(cat ./data/restart 2>/dev/null || echo 0) + 1 )) > ./data/restart
status:
	@cat ./data/start 2>/dev/null || echo 0
migrate:
	mkdir -p ./data
	@echo migrated > ./data/migrate
`
}

func writeVersionedIntegrationJob(t *testing.T, version string) {
	t.Helper()
	jobDir := filepath.Join(bucket.WorkspaceLocation, "jobs", integrationJobName)
	require.NoError(t, os.MkdirAll(jobDir, 0o755))

	versionField := ""
	if version != "" {
		versionField = fmt.Sprintf(`"version": %q,`, version)
	}
	manifest := fmt.Sprintf(`{%s "selectors": ["worker"]}`, versionField)
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "manifest.json"), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "Makefile"), []byte(versionTrackingMakefile()), 0o644))
}

func setupVersionIntegrationBucket(t *testing.T, version string) {
	t.Helper()
	requireIntegrationAssets(t)
	resetIntegrationBucket(t)
	require.NoError(t, initialize.Execute())
	installAssets(t)
	writeVersionedIntegrationJob(t, version)
	require.NoError(t, build.Execute())
}

func latestKVValue(t *testing.T, namespace, key string) string {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var value string
	var deleted int
	err = db.QueryRow(`
		SELECT value, deleted FROM key_value
		WHERE namespace = ? AND key = ?
		ORDER BY version DESC LIMIT 1`,
		namespace, key,
	).Scan(&value, &deleted)
	require.NoError(t, err)
	require.NotEqual(t, 1, deleted)
	return value
}

func jobCatalogVersion(t *testing.T, job string) string {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var version string
	require.NoError(t, db.QueryRow(`SELECT version FROM job WHERE name = ?`, job).Scan(&version))
	return version
}

func allocationHashVersions(t *testing.T, job, workerIP string) (currentVersion, newVersion string) {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	allocID, err := data.GetAllocationID(tx, workerIP, job)
	require.NoError(t, err)

	namespace := fmt.Sprintf("%s_allocation", job)
	var current, newVer sql.NullString
	err = tx.QueryRow(
		`SELECT current_version, new_version FROM hash WHERE namespace = ? AND key = ?`,
		namespace, allocID,
	).Scan(&current, &newVer)
	require.NoError(t, err)

	if current.Valid && current.String != "" {
		currentVersion = current.String
	} else {
		currentVersion = data.DefaultAllocationVersion
	}
	if newVer.Valid && newVer.String != "" {
		newVersion = newVer.String
	} else {
		newVersion = data.DefaultAllocationVersion
	}
	return currentVersion, newVersion
}

func jobAllocationVersionsPromoted(t *testing.T, job string) bool {
	t.Helper()
	for _, workerIP := range workerIPs(t) {
		current, newVer := allocationHashVersions(t, job, workerIP)
		if current != newVer {
			return false
		}
	}
	return true
}

func workerVersionData(t *testing.T, workerIP, field string) string {
	t.Helper()
	bucketID := mustBucketID(t)
	path := fmt.Sprintf(
		"/opt/worker/%s/jobs/%s/data/%s",
		bucketID, integrationJobName, field,
	)
	return strings.TrimSpace(remoteShellOutput(t, workerIP, fmt.Sprintf("cat %s 2>/dev/null || echo", path)))
}
