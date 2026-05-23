package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/jobcommand"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeBunJob(t *testing.T, jobName, commandName, ext, script string, executedOn []string) {
	t.Helper()

	eventsJSON := `["cli"]`
	if len(executedOn) > 0 {
		eventsJSON = `[`
		for i, e := range executedOn {
			if i > 0 {
				eventsJSON += ","
			}
			eventsJSON += `"` + e + `"`
		}
		eventsJSON += `]`
	}

	manifest := `{
		"selectors": ["worker"],
		"commands": {
			"` + commandName + `": {
				"executed_on": ` + eventsJSON + `,
				"demands": {"job": "", "command": ""}
			}
		}
	}`
	writeMinimalJob(t, jobName, manifest)

	moduleDir := path.Join(bucket.WorkspaceLocation, "jobs", jobName, "_modules")
	require.NoError(t, os.WriteFile(path.Join(moduleDir, commandName+ext), []byte(script), 0o644))
}

func TestBunJobCommandStoredInJobFiles(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeBunJob(t, "bunjob", "command_health_check", ".ts", `export {}`, nil)

	runBuild(t)

	var content string
	MustQueryRow(t,
		`SELECT content FROM job_files WHERE path = 'bunjob/_modules/command_health_check.ts'`,
		&content,
	)
	assert.Equal(t, "export {}", content)
}

func TestWorkerModuleLayoutResolvesBunAfterCopy(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeBunJob(t, "bunjob", "command_deploy", ".ts", `export {}`, []string{"cli"})

	runBuild(t)

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	require.NoError(t, kv.Initialize(tx))

	workerIP := "10.0.0.1"
	workerDir := bucket.GetTempWorkerPath(workerIP)
	require.NoError(t, os.MkdirAll(path.Join(workerDir, "jobs"), 0o755))
	require.NoError(t, data.CopyJobFiles(tx, "bunjob", path.Join(workerDir, "jobs")))

	moduleDir := path.Join(workerDir, "jobs", "bunjob", "_modules")
	require.NoError(t, os.WriteFile(path.Join(moduleDir, "maand.py"), jobcommand.MaandPy, 0o644))
	require.NoError(t, os.WriteFile(path.Join(moduleDir, "maand.ts"), jobcommand.MaandTS, 0o644))

	runtime, scriptPath, err := jobcommand.ResolveCommandScript(moduleDir, "command_deploy")
	require.NoError(t, err)
	assert.Equal(t, jobcommand.RuntimeBun, runtime)
	assert.Equal(t, path.Join(moduleDir, "command_deploy.ts"), scriptPath)
}
