package tests

import (
	"fmt"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/cat"
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

func TestJobDisabled(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), os.ModePerm)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors": ["worker"]}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	var count int
	count = GetRowCount("SELECT count(1) FROM allocations WHERE disabled = 0")
	assert.Equal(t, 1, count)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "disabled.json"), []byte(`{"jobs":{"a":{}}}`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(1) FROM allocations WHERE disabled = 0")
	assert.Equal(t, 0, count)
	count = GetRowCount("SELECT count(1) FROM cat_kv WHERE deleted = 0")
	assert.Equal(t, 22, count)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "disabled.json"), []byte(`{"jobs":{}}`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(1) FROM allocations WHERE disabled = 0")
	assert.Equal(t, 1, count)
}

func TestAllocationDisabled(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)
	err := initialize.Execute()
	assert.NoError(t, err)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), os.ModePerm)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors": ["worker"]}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	var count int
	count = GetRowCount("SELECT count(1) FROM allocations WHERE disabled = 0")
	assert.Equal(t, 1, count)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "disabled.json"), []byte(`{"jobs":{"a":{"workers":["10.0.0.1"]}}}`), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)
	count = GetRowCount("SELECT count(1) FROM allocations WHERE disabled = 0")
	assert.Equal(t, 0, count)

	count = GetRowCount("SELECT count(1) FROM cat_kv WHERE deleted = 0")
	assert.Equal(t, 22, count)

	// jobs is still disabled
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "disabled.json"), []byte(`{"jobs":{"a":{"workers":[]}}}`), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)
	count = GetRowCount("SELECT count(1) FROM allocations WHERE disabled = 0")
	assert.Equal(t, 0, count)
	count = GetRowCount("SELECT count(1) FROM cat_kv WHERE deleted = 0")
	assert.Equal(t, 22, count)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "disabled.json"), []byte(`{"jobs":{}}`), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)
	count = GetRowCount("SELECT count(1) FROM allocations WHERE disabled = 0")
	assert.Equal(t, 1, count)
}

func TestWorkerDisabled(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)
	err := initialize.Execute()
	assert.NoError(t, err)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), os.ModePerm)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors": ["worker"]}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	var count int
	count = GetRowCount("SELECT count(1) FROM allocations WHERE disabled = 0")
	assert.Equal(t, 1, count)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "disabled.json"), []byte(`{"workers":["10.0.0.1"]}`), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(1) FROM allocations WHERE disabled = 0")
	assert.Equal(t, 0, count)
}

func TestJobMinSetToMaxIfNotFound(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"cpu": {"min":"12 Ghz"}, "memory":{"min":"12 GB"}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("maand/job/a", "min_cpu_mhz")
	assert.Equal(t, "12000", value)
	value, _ = GetKey("maand/job/a", "max_cpu_mhz")
	assert.Equal(t, "12000", value)

	value, _ = GetKey("maand/job/a", "min_memory_mb")
	assert.Equal(t, "12288", value)
	value, _ = GetKey("maand/job/a", "max_memory_mb")
	assert.Equal(t, "12288", value)
}

func TestJobMaxCPUGreatherThanMin(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"cpu": {"min":"12", "max": "12"}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"cpu": {"min":"12", "max": "13"}}}`), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"cpu": {"min":"12", "max": "11"}}}`), os.ModePerm)
	err = build.Execute()
	assert.Error(t, err)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"memory": {"min":"12", "max": "12"}}}`), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"memory": {"min":"12", "max": "13"}}}`), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"memory": {"min":"12", "max": "11"}}}`), os.ModePerm)
	err = build.Execute()
	assert.Error(t, err)
}

func TestJobMaxMemoryAsDefaultIfNotInJobConf(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1", "memory": "12"}]`), os.ModePerm)
	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"], "resources":{"cpu": {"min":"12", "max": "12"}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("maand/job/a", "cpu")
	assert.Equal(t, "12", value)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"], "resources":{"memory": {"min":"12", "max": "12"}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("maand/job/a", "memory")
	assert.Equal(t, "12", value)

	err = cat.KV()
	assert.NoError(t, err)
}

func TestJobPortCollisionWithinJob(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"ports": {"a_port_web":4545, "a_port_web1":4545}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.Error(t, err)
}

// test job resources port collision
func TestJobPortCollisionWithDifferentJob(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"ports": {"a_port_web":4545}}}`), os.ModePerm)

	jobPath = path.Join(bucket.WorkspaceLocation, "jobs", "b")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"ports": {"b_port_web":4545}}}`), os.ModePerm)

	err = build.Execute()
	assert.Error(t, err)
}

func TestJobKVCountJobRemovedNoWorker(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors": ["worker"]}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT count(*) FROM cat_kv where deleted = 0")
	assert.Equal(t, 11, count)

	_ = os.RemoveAll(path.Join(bucket.WorkspaceLocation, "jobs", "a"))

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(*) FROM cat_kv where deleted = 0")
	assert.Equal(t, 1, count)
}

func TestJobKVCountJobRemoved(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), os.ModePerm)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors": ["worker"]}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT count(*) FROM cat_kv where deleted = 0")
	assert.Equal(t, 22, count)

	_ = os.RemoveAll(path.Join(bucket.WorkspaceLocation, "jobs", "a"))

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(*) FROM cat_kv where deleted = 0")
	assert.Equal(t, 12, count)
}

func TestJobKVCountWorkerJSONRemovedLater(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT count(*) FROM cat_kv where deleted = 0")
	assert.Equal(t, 11, count)

	_ = os.RemoveAll(path.Join(bucket.WorkspaceLocation, "workers.json"))
	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(*) FROM cat_kv where deleted = 0")
	assert.Equal(t, 1, count)

	err = cat.KV()
	assert.NoError(t, err)
}

func TestCountKVWorkerJSONAndJobRemoved(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), os.ModePerm)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors": ["worker"]}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT count(*) FROM cat_kv where deleted = 0")
	assert.Equal(t, 22, count)

	_ = os.RemoveAll(path.Join(bucket.WorkspaceLocation, "workers.json"))
	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(*) FROM cat_kv where deleted = 0")
	assert.Equal(t, 11, count)

	_ = os.RemoveAll(path.Join(bucket.WorkspaceLocation, "jobs", "a"))
	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(*) FROM cat_kv where deleted = 0")
	assert.Equal(t, 1, count)
}

func TestJobKVMemoryCPUJobConf(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	jobsConf := `
[a]
memory="12"
cpu="112"
`
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(jobsConf), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("maand/job/a", "memory")
	assert.Equal(t, "12", value)

	value, _ = GetKey("vars/bucket/job/a", "memory")
	assert.Equal(t, "12", value)

	value, _ = GetKey("maand/job/a", "min_memory_mb")
	assert.Equal(t, "12", value)

	value, _ = GetKey("maand/job/a", "max_memory_mb")
	assert.Equal(t, "12", value)

	value, _ = GetKey("maand/job/a", "cpu")
	assert.Equal(t, "112", value)

	value, _ = GetKey("maand/job/a", "min_cpu_mhz")
	assert.Equal(t, "112", value)

	value, _ = GetKey("maand/job/a", "max_cpu_mhz")
	assert.Equal(t, "112", value)

	value, _ = GetKey("vars/bucket/job/a", "cpu")
	assert.Equal(t, "112", value)

	err = cat.KV()
	assert.NoError(t, err)
}

func TestJobKVMaxMemoryAsDefaultIfNotInJobConf(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1", "memory": "12"}]`), os.ModePerm)
	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"], "resources":{"cpu": {"min":"12", "max": "12"}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("maand/job/a", "cpu")
	assert.Equal(t, "12", value)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"], "resources":{"memory": {"min":"11", "max": "12"}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("maand/job/a", "memory")
	assert.Equal(t, "12", value)

	err = cat.KV()
	assert.NoError(t, err)
}

func TestJobConfMemoryCPUConv(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	jobsConf := `
[a]
memory="1 GB"
cpu="1 GHZ"
`
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(jobsConf), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("maand/job/a", "memory")
	assert.Equal(t, "1024", value)

	value, _ = GetKey("vars/bucket/job/a", "memory")
	assert.Equal(t, "1 GB", value)

	value, _ = GetKey("maand/job/a", "cpu")
	assert.Equal(t, "1000", value)

	value, _ = GetKey("vars/bucket/job/a", "cpu")
	assert.Equal(t, "1 GHZ", value)
}

func TestJobMakeFile(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"]}`), os.ModePerm)
	err = build.Execute()

	assert.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidJob)
}

func TestJobInsufficientMemoryJobConf(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1", "memory": "12 mb"}]`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"]}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	jobsConf := `
[a]
memory="1 GB"
cpu="1 GHZ"
	`
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf"), []byte(jobsConf), os.ModePerm)
	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrInSufficientResource)
}

func TestJobInsufficientMemoryManifest(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1", "memory": "12 mb"}]`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"], "resources": { "memory":{ "min":"1GB" }}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrInSufficientResource)
}

func TestJobInsufficientMemoryManifest1(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1", "memory": "12gb"}]`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"], "resources": { "memory":{ "min":"1GB", "max": "22gb" }}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrInSufficientResource)
}

func TestJobDepsAndConfig(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	createJob("a")
	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	modulesDir := path.Join(jobDir, "_modules")
	_ = os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
	"selectors": ["worker"],
		"commands": {
			"command_test_command": {
				"executed_on": ["post_deploy"]
			}
		}
	}`), 0o644)

	_ = os.MkdirAll(modulesDir, 0o755)
	_ = os.WriteFile(path.Join(modulesDir, "command_test_command.py"), []byte(``), 0o755)

	createJob("b")
	jobDir = path.Join(bucket.WorkspaceLocation, "jobs", "b")
	modulesDir = path.Join(jobDir, "_modules")
	_ = os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
	"selectors": ["worker"],
		"commands": {
			"command_test_command": {
				"executed_on": ["post_deploy"],
				"demands": {
					"job": "a",
					"command": "command_test_command",
					"config": {"a":1}
				}
			}
		}
	}`), 0o644)

	_ = os.MkdirAll(modulesDir, 0o755)
	_ = os.WriteFile(path.Join(modulesDir, "command_test_command.py"), []byte(``), 0o755)

	err = build.Execute()
	assert.NoError(t, err)

	err = cat.JobCommands()
	assert.NoError(t, err)
}

func TestJobDepsLevel3(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	host1 := "10.0.0.1"
	host2 := "10.0.0.2"
	host3 := "10.0.0.3"

	d := `[{"host":"%s"},{"host":"%s"},{"host":"%s"}]`
	workers := fmt.Sprintf(d, host1, host2, host3)
	_ = os.MkdirAll(bucket.WorkspaceLocation, 0o755)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(workers), 0o755)

	err := initialize.Execute()
	assert.NoError(t, err)

	createJob("a")
	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	modulesDir := path.Join(jobDir, "_modules")
	_ = os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
	"selectors": ["worker"],
		"commands": {
			"command_test_command": {
				"executed_on": ["post_deploy"]
			}
		}
	}`), 0o644)

	_ = os.MkdirAll(modulesDir, 0o755)
	_ = os.WriteFile(path.Join(modulesDir, "command_test_command.py"), []byte(``), 0o755)

	createJob("b")
	jobDir = path.Join(bucket.WorkspaceLocation, "jobs", "b")
	modulesDir = path.Join(jobDir, "_modules")
	_ = os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
	"selectors": ["worker"],
		"commands": {
			"command_test_command": {
				"executed_on": ["post_deploy"],
				"demands": {
					"job": "a",
					"command": "command_test_command"
				}
			}
		}
	}`), 0o644)

	_ = os.MkdirAll(modulesDir, 0o755)
	_ = os.WriteFile(path.Join(modulesDir, "command_test_command.py"), []byte(``), 0o755)

	createJob("c")
	jobDir = path.Join(bucket.WorkspaceLocation, "jobs", "c")
	modulesDir = path.Join(jobDir, "_modules")
	_ = os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
	"selectors": ["worker"],
		"commands": {
			"command_test_command": {
				"executed_on": ["post_deploy"],
				"demands": {
					"job": "b",
					"command": "command_test_command"
				}
			}
		}
	}`), 0o644)

	_ = os.MkdirAll(modulesDir, 0o755)
	_ = os.WriteFile(path.Join(modulesDir, "command_test_command.py"), []byte(``), 0o755)

	createJob("d")
	jobDir = path.Join(bucket.WorkspaceLocation, "jobs", "d")
	modulesDir = path.Join(jobDir, "_modules")
	_ = os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
	"selectors": ["worker"],
		"commands": {
			"command_test_command1": {
				"executed_on": ["post_deploy"],
				"demands": {
					"job": "a",
					"command": "command_test_command"
				}
			},
			"command_test_command2": {
				"executed_on": ["post_deploy"],
				"demands": {
					"job": "b",
					"command": "command_test_command"
				}
			}
		}
	}`), 0o644)

	_ = os.MkdirAll(modulesDir, 0o755)
	_ = os.WriteFile(path.Join(modulesDir, "command_test_command1.py"), []byte(``), 0o755)
	_ = os.WriteFile(path.Join(modulesDir, "command_test_command2.py"), []byte(``), 0o755)

	err = build.Execute()
	assert.NoError(t, err)

	var seq int

	GetRowValues("SELECT deployment_seq FROM cat_jobs where name = 'a'", &seq)
	assert.Equal(t, 0, seq)

	GetRowValues("SELECT deployment_seq FROM cat_jobs where name = 'b'", &seq)
	assert.Equal(t, 1, seq)

	GetRowValues("SELECT deployment_seq FROM cat_jobs where name = 'c'", &seq)
	assert.Equal(t, 2, seq)

	GetRowValues("SELECT deployment_seq FROM cat_jobs where name = 'd'", &seq)
	assert.Equal(t, 2, seq)
}
