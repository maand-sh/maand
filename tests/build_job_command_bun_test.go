package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobCommandBunTypeScript(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)

	manifest := `{
		"selectors": ["worker"],
		"commands": {
			"command_health_check": {
				"executed_on": ["cli"],
				"demands": {"job": "", "command": ""}
			}
		}
	}`
	writeMinimalJob(t, "bunjob", manifest)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "bunjob", "_modules")
	require.NoError(t, os.WriteFile(path.Join(jobPath, "command_health_check.ts"), []byte(`export {}`), 0o644))

	runBuild(t)

	count := GetRowCount("SELECT count(1) FROM job_commands WHERE job = 'bunjob' AND executed_on = 'cli'")
	assert.Equal(t, 1, count)
}

func TestJobCommandBunAndPythonConflict(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)

	manifest := `{
		"selectors": ["worker"],
		"commands": {
			"command_health_check": {
				"executed_on": ["cli"],
				"demands": {"job": "", "command": ""}
			}
		}
	}`
	writeMinimalJob(t, "conflict", manifest)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "conflict", "_modules")
	require.NoError(t, os.WriteFile(path.Join(jobPath, "command_health_check.py"), []byte(``), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "command_health_check.ts"), []byte(``), 0o644))

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrInvalidJobCommandConfiguration)
}

func TestJobCommandBunJavaScript(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeBunJob(t, "jsjob", "command_health_check", ".js", `export {}`, nil)

	runBuild(t)

	count := GetRowCount("SELECT count(1) FROM job_commands WHERE job = 'jsjob'")
	assert.Equal(t, 1, count)
}

func TestJobCommandBunFileMissing(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)

	manifest := `{
		"selectors": ["worker"],
		"commands": {
			"command_health_check": {
				"executed_on": ["cli"],
				"demands": {"job": "", "command": ""}
			}
		}
	}`
	writeMinimalJob(t, "missing", manifest)

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrJobCommandFileNotFound)
}

func TestJobCommandBunPythonAndJSConflict(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)

	manifest := `{
		"selectors": ["worker"],
		"commands": {
			"command_health_check": {
				"executed_on": ["cli"],
				"demands": {"job": "", "command": ""}
			}
		}
	}`
	writeMinimalJob(t, "conflict2", manifest)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "conflict2", "_modules")
	require.NoError(t, os.WriteFile(path.Join(jobPath, "command_health_check.py"), []byte(``), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "command_health_check.js"), []byte(``), 0o644))

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrInvalidJobCommandConfiguration)
}

func TestJobCommandBunEventsUpdatedOnRebuild(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeBunJob(t, "bunevents", "command_health_check", ".ts", `export {}`, []string{"cli"})

	runBuild(t)
	assert.Equal(t, 1, GetRowCount(
		"SELECT count(1) FROM job_commands WHERE job = 'bunevents' AND executed_on = 'cli'",
	))

	manifest := `{
		"selectors": ["worker"],
		"commands": {
			"command_health_check": {
				"executed_on": ["cli", "pre_deploy"],
				"demands": {"job": "", "command": ""}
			}
		}
	}`
	require.NoError(t, os.WriteFile(
		path.Join(bucket.WorkspaceLocation, "jobs", "bunevents", "manifest.json"),
		[]byte(manifest),
		0o644,
	))

	runBuild(t)
	assert.Equal(t, 2, GetRowCount(
		"SELECT count(1) FROM job_commands WHERE job = 'bunevents' AND executed_on IN ('cli', 'pre_deploy')",
	))
}
