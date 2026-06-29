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

func TestPrepareHealthWorkspaceCopiesOnlyCommandModule(t *testing.T) {
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

	jobName := "app"
	jobID := "job-id-app"
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

	files := map[string]string{
		"app":                          "",
		"app/_modules":                 "",
		"app/Makefile":                 "start:\n",
		"app/manifest.json":            `{}`,
		"app/_modules/command_health.py": `print("ok")`,
	}
	for filePath, content := range files {
		isDir := content == ""
		_, err = tx.Exec(
			`INSERT INTO job_files (job_id, path, content, isdir) VALUES (?, ?, ?, ?)`,
			jobID, filePath, []byte(content), isDir,
		)
		requireNoError(t, err)
	}

	_, err = tx.Exec(
		`INSERT INTO key_value (namespace, key, value, version, ttl, created_date, deleted) VALUES (?, ?, ?, 1, '0', ?, 0)`,
		"maand/worker", "certs/ca.crt", "test-ca", time.Now().Unix(),
	)
	requireNoError(t, err)
	requireNoError(t, kv.Initialize(tx))

	workerIP := "10.0.0.1"
	requireNoError(t, prepareHealthWorkerWorkspace(tx, jobName, "command_health", workerIP, kv.GetStore()))

	jobRoot := path.Join(bucket.GetTempWorkerPath(workerIP), "jobs", jobName)
	if _, err := os.Stat(path.Join(jobRoot, "Makefile")); err == nil {
		t.Fatal("health-fast workspace should not copy Makefile")
	}
	if _, err := os.Stat(path.Join(jobRoot, "_modules", "command_health.py")); err != nil {
		t.Fatalf("missing command module: %v", err)
	}
	if _, err := os.Stat(path.Join(jobRoot, "_modules", embeddedMaandPyModule)); err != nil {
		t.Fatalf("missing maand.py: %v", err)
	}
}
