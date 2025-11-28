package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
)

func TestJobCommandWithoutTrigger(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"commands":{"command_health_check":{}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)
	_ = os.MkdirAll(path.Join(jobPath, "_modules"), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "_modules", "command_health_check.py"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrInvalidJobCommandConfiguration)
}

func TestJobCommandBuildWithTrigger(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)
	_ = os.MkdirAll(path.Join(jobPath, "_modules"), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "_modules", "command_health_check.py"), []byte(``), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"commands":{"command_health_check":{"executed_on":["post_build"]}}}`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT count(1) FROM job_commands WHERE executed_on in ('post_build')")
	assert.Equal(t, 1, count)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"commands":{"command_health_check":{"executed_on":["post_build", "pre_deploy"]}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(1) FROM job_commands WHERE executed_on in ('post_build', 'pre_deploy')")
	assert.Equal(t, 2, count)
}

func TestJobCommandFileMissing(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"commands":{"command_health_check":{"executed_on":["post_build"]}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrJobCommandFileNotFound)
}

func TestJobCommandAddedUpdatedRemoved(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(path.Join(jobPath, "_modules"), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "_modules", "command_health_check1.py"), []byte(``), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "_modules", "command_health_check2.py"), []byte(``), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "_modules", "command_health_check3.py"), []byte(``), os.ModePerm)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"commands":{"command_health_check1":{"executed_on":["post_build"]}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT count(1) FROM job_commands WHERE executed_on = 'post_build'")
	assert.Equal(t, 1, count)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"commands":{"command_health_check1":{"executed_on":["post_build"]}, "command_health_check2":{"executed_on":["post_build"]}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(1) FROM job_commands WHERE executed_on = 'post_build'")
	assert.Equal(t, 2, count)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"commands":{"command_health_check3":{"executed_on":["post_build"]}, "command_health_check2":{"executed_on":["post_build"]}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(1) FROM job_commands WHERE executed_on = 'post_build'")
	assert.Equal(t, 2, count)
}

func TestJobDepsAndDeploymentSeq(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(path.Join(jobPath, "_modules"), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "_modules", "command_health_check1.py"), []byte(``), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors": ["worker"], "commands":{"command_health_check1":{"executed_on":["post_build"]}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	jobPath = path.Join(bucket.WorkspaceLocation, "jobs", "b")
	_ = os.MkdirAll(path.Join(jobPath, "_modules"), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "_modules", "command_health_check2.py"), []byte(``), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors": ["worker"], "commands":{"command_health_check2":{"executed_on":["post_build"], "demands":{"job":"a","command":"command_health_check1"}}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	jobPath = path.Join(bucket.WorkspaceLocation, "jobs", "c")
	_ = os.MkdirAll(path.Join(jobPath, "_modules"), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "_modules", "command_health_check3.py"), []byte(``), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors": ["worker"], "commands":{"command_health_check3":{"executed_on":["post_build"], "demands":{"job":"a","command":"command_health_check1"}}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT count(1) FROM job_commands WHERE executed_on = 'post_build'")
	assert.Equal(t, 3, count)

	rows := GetRows("SELECT name, deployment_seq FROM cat_jobs")
	jobs := make(map[string]int)
	for rows.Next() {
		var job string
		var deploymentSeq int
		_ = rows.Scan(&job, &deploymentSeq)
		jobs[job] = deploymentSeq
	}

	assert.Equal(t, jobs, map[string]int{"a": 0, "b": 1, "c": 1})
}

func TestJobCommandPrefixFormat(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(path.Join(jobPath, "_modules"), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "_modules", "1command_health_check1.py"), []byte(``), os.ModePerm)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"commands":{"1command_health_check1":{"executed_on":["post_build"]}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrInvalidJobCommandConfiguration)
}
