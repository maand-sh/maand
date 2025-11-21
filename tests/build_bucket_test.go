package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/cat"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	err = build.Execute()
	assert.NoError(t, err)
}

func TestInitKV(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	err = build.Execute()
	assert.NoError(t, err)

	err = cat.KV()
	assert.NoError(t, err)

	key, _ := GetKey("maand/worker", "certs/ca.crt")
	assert.NotEmpty(t, key)
}

func TestUntouchedBucketConf(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	_ = os.MkdirAll(bucket.WorkspaceLocation, os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.conf"), []byte(`a="1"`), os.ModePerm)

	err := initialize.Execute()
	assert.NoError(t, err)

	err = build.Execute()
	assert.NoError(t, err)

	key, _ := GetKey("vars/bucket", "a")
	assert.Equal(t, "1", key)
}

func TestBucketConfOps(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.conf"), []byte(`a="1"`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("vars/bucket", "a")
	assert.Equal(t, "1", value)

	vars := `
a="3"
b="1"
`
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.conf"), []byte(vars), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("vars/bucket", "a")
	assert.Equal(t, "3", value)
	value, _ = GetKey("vars/bucket", "b")
	assert.Equal(t, "1", value)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.conf"), []byte(`a="1"`), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("vars/bucket", "a")
	assert.Equal(t, "1", value)
	value, _ = GetKey("vars/bucket", "b")
	assert.Equal(t, "", value)

	_ = os.Remove(path.Join(bucket.WorkspaceLocation, "bucket.conf"))

	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("vars/bucket", "a")
	assert.Equal(t, "", value)

	kvcount := GetRowCount("select count(1) from key_value WHERE deleted = 1;")
	assert.Equal(t, 2, kvcount)
}

// test bucket.conf empty build
func TestEmptyBucketConf(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.conf"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	key, _ := GetKey("maand", "a")
	assert.Equal(t, "", key)

	kvcount := GetRowCount("select count(1) from key_value")
	assert.Equal(t, 1, kvcount)
}

func TestBucketConfAdded(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	err = build.Execute()
	assert.NoError(t, err)

	key, _ := GetKey("maand", "a")
	assert.Equal(t, "", key)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.conf"), []byte(`a="1"`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	key, _ = GetKey("vars/bucket", "a")
	assert.Equal(t, "1", key)
}

func TestInitFiles(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	assert.FileExists(t, path.Join(bucket.WorkspaceLocation, "workers.json"))
	assert.DirExists(t, path.Join(bucket.WorkspaceLocation, "jobs"))
	assert.FileExists(t, path.Join(bucket.WorkspaceLocation, "bucket.conf"))
	assert.FileExists(t, path.Join(bucket.Location, "maand.conf"))
	assert.DirExists(t, path.Join(bucket.Location, "secrets"))
	assert.FileExists(t, path.Join(bucket.Location, "secrets", "ca.key"))
	assert.FileExists(t, path.Join(bucket.Location, "secrets", "ca.crt"))
	assert.DirExists(t, path.Join(bucket.Location, "logs"))
	assert.DirExists(t, path.Join(bucket.Location, "data"))
	assert.FileExists(t, path.Join(bucket.Location, "data", "maand.db"))
	assert.DirExists(t, path.Join(bucket.WorkspaceLocation))
	assert.DirExists(t, path.Join(bucket.WorkspaceLocation, "docker"))
	assert.FileExists(t, path.Join(bucket.WorkspaceLocation, "docker", "Dockerfile"))
	assert.FileExists(t, path.Join(bucket.WorkspaceLocation, "docker", "requirements.txt"))
}

func TestJobConfAddedLater(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"version":"1.1"}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("maand/job/a", "test")
	assert.Equal(t, "", value)

	jobsConf := `
[a]
test="1"
`
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(jobsConf), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("vars/bucket/job/a", "test")
	assert.Equal(t, "1", value)
}

func TestJobConfEmptyLater(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"version":"1.1"}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	jobsConf := ``
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(jobsConf), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)
}

func TestJobConfUntouched(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	_ = os.MkdirAll(bucket.WorkspaceLocation, os.ModePerm)
	jobsConf := `
[a]
test="1"
`
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(jobsConf), os.ModePerm)
	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"version":"1.1"}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("vars/bucket/job/a", "test")
	assert.Equal(t, "1", value)
}

func TestJobKV(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"version":"1.1"}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("maand/job/a", "job_id")
	assert.NotEmpty(t, value)

	value, _ = GetKey("maand/job/a", "max_cpu_mhz")
	assert.Equal(t, "0", value)
	value, _ = GetKey("maand/job/a", "max_memory_mb")
	assert.Equal(t, "0", value)

	value, _ = GetKey("maand/job/a", "min_cpu_mhz")
	assert.Equal(t, "0", value)
	value, _ = GetKey("maand/job/a", "min_memory_mb")
	assert.Equal(t, "0", value)

	value, _ = GetKey("maand/job/a", "name")
	assert.Equal(t, "a", value)
	value, _ = GetKey("maand/job/a", "version")
	assert.Equal(t, "1.1", value)

	value, _ = GetKey("maand/job/a", "memory")
	assert.Equal(t, "0", value)
	value, _ = GetKey("maand/job/a", "cpu")
	assert.Equal(t, "0", value)
}
