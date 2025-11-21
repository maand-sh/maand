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

func TestJobWithoutManifest(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.MkdirAll(path.Join(bucket.WorkspaceLocation, "jobs", "a"), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT COUNT(1) FROM job")
	assert.Equal(t, 0, count)
}

func TestJob(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT COUNT(1) FROM job")
	assert.Equal(t, 1, count)

	jobPath = path.Join(bucket.WorkspaceLocation, "jobs", "b")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT COUNT(1) FROM job")
	assert.Equal(t, 2, count)
}

func TestJobRemoved(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	jobPath = path.Join(bucket.WorkspaceLocation, "jobs", "b")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT COUNT(1) FROM job")
	assert.Equal(t, 2, count)

	_ = os.RemoveAll(path.Join(bucket.WorkspaceLocation, "jobs", "b"))
	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT COUNT(1) FROM job")
	assert.Equal(t, 1, count)
}

func TestJobBuildUpdateJobFiles(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)
	_ = os.MkdirAll(path.Join(jobPath, "test"), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "test", "test"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT COUNT(1) FROM job")
	assert.Equal(t, 1, count)

	count = GetRowCount("SELECT COUNT(1) FROM job_files")
	assert.Equal(t, 5, count)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"version":"1.1"}`), os.ModePerm)
	_ = os.RemoveAll(path.Join(jobPath, "test"))

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT COUNT(1) FROM job_files")
	assert.Equal(t, 3, count)

	var content string
	GetRowValues("SELECT content FROM job_files where path = 'a/manifest.json'", &content)
	assert.Equal(t, `{"version":"1.1"}`, content)
}

func TestJobVersionUpdate(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"version":"1.1"}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	var version string
	GetRowValues("SELECT version FROM job", &version)
	assert.Equal(t, "1.1", version)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"version":"1.2"}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues("SELECT version FROM job", &version)
	assert.Equal(t, "1.2", version)
}

func TestJobConf(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"version":"1.1"}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	jobsConf := `
[a]
test="1"
var="1"
`
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(jobsConf), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("vars/bucket/job/a", "test")
	assert.Equal(t, "1", value)

	value, _ = GetKey("vars/bucket/job/a", "var")
	assert.Equal(t, "1", value)

	jobsConf = `
[a]
test="2"
`
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(jobsConf), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("vars/bucket/job/a", "test")
	assert.Equal(t, "2", value)

	value, _ = GetKey("vars/bucket/job/a", "var")
	assert.Equal(t, "", value)

	_ = os.Remove(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"))

	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("vars/bucket/job/a", "test")
	assert.Equal(t, "", value)

	value, _ = GetKey("vars/bucket/job/a", "var")
	assert.Equal(t, "", value)
}

func TestJobConfInvalid(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"version":"1.1"}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	jobsConf := `
[a]
test=1
`
	// expected as string value

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(jobsConf), os.ModePerm)
	err = build.Execute()

	assert.ErrorIs(t, err, bucket.ErrInvalidBucketConf)
}
