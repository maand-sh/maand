package jobcommand

import (
	"os"
	"path"
	"testing"
	"time"

	"maand/bucket"
	"maand/data"
	"maand/initialize"
	"maand/kv"
)

func TestPrepareWorkerWorkspaceWritesSDKAndBunScript(t *testing.T) {
	root := t.TempDir()
	origLocation := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = origLocation
		bucket.UpdatePath()
	})

	requireNoError(t, initialize.Execute())

	db, err := data.OpenDatabase(true)
	requireNoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	tx, err := db.Begin()
	requireNoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	requireNoError(t, kv.Initialize(tx))

	jobName := "bunapp"
	jobID := "job-id-bun"
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			max_concurrent_upgrades
		) VALUES (?, ?, '1', '0', '0', '0', '0', '0', '0', 1)`,
		jobID, jobName,
	)
	requireNoError(t, err)

	for _, dirPath := range []string{"bunapp", "bunapp/_modules"} {
		_, err = tx.Exec(
			`INSERT INTO job_files (job_id, path, content, isdir) VALUES (?, ?, '', 1)`,
			jobID, dirPath,
		)
		requireNoError(t, err)
	}

	scriptPath := "bunapp/_modules/command_health_check.ts"
	scriptContent := `import { jobName } from "./maand"; console.log(jobName());`
	_, err = tx.Exec(
		`INSERT INTO job_files (job_id, path, content, isdir) VALUES (?, ?, ?, 0)`,
		jobID, scriptPath, []byte(scriptContent),
	)
	requireNoError(t, err)

	_, err = tx.Exec(
		`INSERT INTO key_value (namespace, key, value, version, ttl, created_date, deleted) VALUES (?, ?, ?, 1, '0', ?, 0)`,
		"maand/worker", "certs/ca.crt", "test-ca", time.Now().Unix(),
	)
	requireNoError(t, err)
	requireNoError(t, kv.Initialize(tx))

	workerIP := "10.0.0.1"
	requireNoError(t, prepareOneWorkerWorkspace(tx, jobName, workerIP, kv.GetStore()))

	moduleDir := path.Join(bucket.GetTempWorkerPath(workerIP), "jobs", jobName, "_modules")

	for _, name := range []string{embeddedMaandPyModule, embeddedMaandTSModule, "command_health_check.ts"} {
		if _, err := os.Stat(path.Join(moduleDir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}

	got, err := os.ReadFile(path.Join(moduleDir, "command_health_check.ts"))
	requireNoError(t, err)
	if string(got) != scriptContent {
		t.Fatalf("script content mismatch: %q", got)
	}

	runtime, resolved, err := ResolveCommandScript(moduleDir, "command_health_check")
	requireNoError(t, err)
	if runtime != RuntimeBun {
		t.Fatalf("runtime %q, want bun", runtime)
	}
	if resolved != path.Join(moduleDir, "command_health_check.ts") {
		t.Fatalf("resolved %q", resolved)
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
