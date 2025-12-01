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

func TestJobMemoryCPUOpsKV(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"cpu": {"min":"12 Ghz", "max":"23 Ghz"}, "memory":{"min":"12 GB", "max":"24 GB"}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("maand/job/a", "min_cpu_mhz")
	assert.NotEmpty(t, "12000", value)
	value, _ = GetKey("maand/job/a", "max_cpu_mhz")
	assert.NotEmpty(t, "23000", value)

	value, _ = GetKey("maand/job/a", "min_memory_mb")
	assert.NotEmpty(t, "12288", value)
	value, _ = GetKey("maand/job/a", "max_memory_mb")
	assert.NotEmpty(t, "24576", value)

	var minMemory, maxMemory, minCPU, maxCPU string
	GetRowValues("SELECT min_memory_mb, max_memory_mb, min_cpu_mhz, max_cpu_mhz FROM job", &minMemory, &maxMemory, &minCPU, &maxCPU)
	assert.Equal(t, "12288", minMemory)
	assert.Equal(t, "24576", maxMemory)
	assert.Equal(t, "12000", minCPU)
	assert.Equal(t, "23000", maxCPU)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"cpu": {"min":"12", "max":"23"}, "memory":{"min":"12", "max":"24"}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("maand/job/a", "min_cpu_mhz")
	assert.NotEmpty(t, "12", value)
	value, _ = GetKey("maand/job/a", "max_cpu_mhz")
	assert.NotEmpty(t, "23", value)

	value, _ = GetKey("maand/job/a", "min_memory_mb")
	assert.NotEmpty(t, "12", value)
	value, _ = GetKey("maand/job/a", "max_memory_mb")
	assert.NotEmpty(t, "24", value)

	GetRowValues("SELECT min_memory_mb, max_memory_mb, min_cpu_mhz, max_cpu_mhz FROM job", &minMemory, &maxMemory, &minCPU, &maxCPU)
	assert.Equal(t, "12", minMemory)
	assert.Equal(t, "24", maxMemory)
	assert.Equal(t, "12", minCPU)
	assert.Equal(t, "23", maxCPU)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("maand/job/a", "min_cpu_mhz")
	assert.NotEmpty(t, "0", value)
	value, _ = GetKey("maand/job/a", "max_cpu_mhz")
	assert.NotEmpty(t, "0", value)

	value, _ = GetKey("maand/job/a", "min_memory_mb")
	assert.NotEmpty(t, "0", value)
	value, _ = GetKey("maand/job/a", "max_memory_mb")
	assert.NotEmpty(t, "0", value)
}

func TestJobPortOpsKV(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"ports": {"a_port_web": 4545}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("maand", "a_port_web")
	assert.Equal(t, "4545", value)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"ports": {"a_port_web": 4547, "a_port_web2": 4546}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("maand", "a_port_web")
	assert.Equal(t, "4547", value)
	value, _ = GetKey("maand", "a_port_web2")
	assert.Equal(t, "4546", value)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"ports": {"a_port_web2": 4546}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("maand", "a_port_web")
	assert.Equal(t, "", value)
	value, _ = GetKey("maand", "a_port_web2")
	assert.Equal(t, "4546", value)
}

func TestJobUniquePort(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"ports": {"a_port_web": 4545}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	jobPath = path.Join(bucket.WorkspaceLocation, "jobs", "b")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"ports": {"b_port_web": 4545}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrPortCollision)

	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"ports": {"b_port_web": 4546}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)
}

func TestJobPortPrefixFormat(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"ports": {"a_port_web": 4546}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	jobPath = path.Join(bucket.WorkspaceLocation, "jobs", "b")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"ports": {"b_port_web": 4545}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"resources":{"ports": {"port_web": 4546}}}`), os.ModePerm)

	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrPortKeyFormat)
}

func TestJobSelectorsAndOperator(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"]}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	err = cat.Allocations("", "")
	assert.NoError(t, err)

	count := GetRowCount("SELECT count(1) FROM allocations where job = 'a' and worker_ip = '10.0.0.1'")
	assert.Equal(t, 1, count)

	count = GetRowCount("SELECT count(1) FROM allocations where job = 'a'")
	assert.Equal(t, 1, count)

	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker", "a"]}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(1) FROM allocations WHERE removed = 1")
	assert.Equal(t, 1, count)
	count = GetRowCount("SELECT count(1) FROM allocations")
	assert.Equal(t, 1, count)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1", "labels": ["a"]}]`), os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(1) FROM allocations WHERE removed = 0")
	assert.Equal(t, 1, count)
	count = GetRowCount("SELECT count(1) FROM allocations")
	assert.Equal(t, 1, count)
}

func TestCAfilesUntouched(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	_ = os.MkdirAll(bucket.SecretLocation, os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.SecretLocation, "ca.key"), []byte(``), os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.SecretLocation, "ca.crt"), []byte(``), os.ModePerm)

	err := initialize.Execute()
	assert.NoError(t, err)

	caKeyFile, _ := os.Stat(path.Join(bucket.SecretLocation, "ca.key"))
	caCrtFile, _ := os.Stat(path.Join(bucket.SecretLocation, "ca.crt"))

	assert.Equal(t, int64(0), caKeyFile.Size())
	assert.Equal(t, int64(0), caCrtFile.Size())
}

func TestAllocations(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "labels": ["a"] },
		{ "host": "10.0.0.2", "labels": ["a"] }
	]`), os.ModePerm)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "jobs", "a", "manifest.json"), []byte(`{
		"selectors": ["a"]
	}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT count(1) FROM allocations WHERE removed = 0")
	assert.Equal(t, 2, count)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "labels": ["a"] },
		{ "host": "10.0.0.2" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(1) FROM allocations WHERE removed = 0")
	assert.Equal(t, 1, count)

	count = GetRowCount("SELECT count(1) FROM allocations WHERE removed = 1")
	assert.Equal(t, 1, count)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "labels": ["a"] },
		{ "host": "10.0.0.2", "labels": ["a"] }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(1) FROM allocations WHERE removed = 0")
	assert.Equal(t, 2, count)

	count = GetRowCount("SELECT count(1) FROM allocations WHERE removed = 1")
	assert.Equal(t, 0, count)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "labels": ["a"] },
		{ "host": "10.0.0.2", "labels": ["a"] },
		{ "host": "10.0.0.3", "labels": ["b"] }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT count(1) FROM allocations WHERE removed = 0")
	assert.Equal(t, 2, count)
}

func TestCatCommand(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host": "10.0.0.1"}]`), os.ModePerm)

	var jobPath string
	jobPath = path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(path.Join(jobPath, "_modules"), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "_modules", "command_health_check.py"), []byte(``), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors": ["worker"], "commands":{"command_health_check":{"executed_on":["post_build"]}}, "resources":{"ports":{"a_port_web":4545}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	jobPath = path.Join(bucket.WorkspaceLocation, "jobs", "b")
	_ = os.MkdirAll(path.Join(jobPath, "_modules"), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "_modules", "command_health_check.py"), []byte(``), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors": ["worker"], "commands":{"command_health_check":{"executed_on":["post_build"]}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	err = cat.JobCommands()
	assert.NoError(t, err)

	err = cat.Jobs()
	assert.NoError(t, err)

	err = cat.JobPorts()
	assert.NoError(t, err)

	err = cat.KV()
	assert.NoError(t, err)

	err = cat.Allocations("", "")
	assert.NoError(t, err)

	err = cat.Workers()
	assert.NoError(t, err)
}

func TestBuildUnInitialized(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	_ = os.MkdirAll(bucket.WorkspaceLocation, os.ModePerm)
	err := build.Execute()
	assert.ErrorIs(t, err, bucket.ErrNotInitialized)
}

func TestCAUpdateUpdatesHash(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	pub, _ := os.ReadFile(path.Join(bucket.SecretLocation, "ca.crt"))
	pri, _ := os.ReadFile(path.Join(bucket.SecretLocation, "ca.key"))

	_ = os.RemoveAll(bucket.Location)
	err = initialize.Execute()
	assert.NoError(t, err)

	err = build.Execute()
	assert.NoError(t, err)

	var currentHash1, previousHash1 string
	GetRowValues("SELECT current_hash, previous_hash from hash WHERE namespace = 'build' AND key = 'ca'", &currentHash1, &previousHash1)

	_ = os.WriteFile(path.Join(bucket.SecretLocation, "ca.crt"), pub, os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.SecretLocation, "ca.key"), pri, os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	var currentHash2, previousHash2 string
	GetRowValues("SELECT current_hash, previous_hash from hash WHERE namespace = 'build' AND key = 'ca'", &currentHash2, &previousHash2)

	assert.NotEqual(t, currentHash1, currentHash2)
	assert.NotEqual(t, previousHash1, previousHash2)
}

func TestCAUpdateTriggerCertUpdates(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	caPub, _ := os.ReadFile(path.Join(bucket.SecretLocation, "ca.crt"))
	caPri, _ := os.ReadFile(path.Join(bucket.SecretLocation, "ca.key"))

	_ = os.RemoveAll(bucket.Location)
	err = initialize.Execute()
	assert.NoError(t, err)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), os.ModePerm)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"], "certs":{"a":{"subject":{"common_name":"3"}}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value1, _ := GetKey("maand/job/a/worker/10.0.0.1", "certs/a.crt")
	assert.NotEmpty(t, value1)

	_ = os.WriteFile(path.Join(bucket.SecretLocation, "ca.crt"), caPub, os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.SecretLocation, "ca.key"), caPri, os.ModePerm)
	err = build.Execute()
	assert.NoError(t, err)

	value2, _ := GetKey("maand/job/a/worker/10.0.0.1", "certs/a.crt")
	assert.NotEmpty(t, value2)

	assert.NotEqual(t, value1, value2)
}

func TestCAAndCertNotUpdatedOnEveryBuild(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), os.ModePerm)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"], "certs":{"a":{"subject":{"common_name":"3"}}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value1, _ := GetKey("maand/job/a/worker/10.0.0.1", "certs/a.crt")
	assert.NotEmpty(t, value1)

	err = build.Execute()
	assert.NoError(t, err)
	err = build.Execute()
	assert.NoError(t, err)

	value2, _ := GetKey("maand/job/a/worker/10.0.0.1", "certs/a.crt")
	assert.NotEmpty(t, value2)

	assert.Equal(t, value1, value2)
}

func TestCertInKV(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), os.ModePerm)
	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"], "certs":{"a":{"subject":{"common_name":"3"}}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value2, _ := GetKey("maand/job/a/worker/10.0.0.1", "certs/a.crt")
	assert.NotEmpty(t, value2)
	value1, _ := GetKey("maand/job/a/worker/10.0.0.1", "certs/a.key")
	assert.NotEmpty(t, value1)
}

func TestCertUpdateAfterExpire(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), os.ModePerm)
	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"], "certs":{"a":{"subject":{"common_name":"3"}}}}`), os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "Makefile"), []byte(``), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value2, _ := GetKey("maand/job/a/worker/10.0.0.1", "certs/a.crt")
	assert.NotEmpty(t, value2)

	conf, err := bucket.GetMaandConf()
	assert.NoError(t, err)

	conf.CertsRenewalBuffer = -60
	_ = os.Remove(path.Join(bucket.Location, "maand.conf"))

	err = bucket.WriteMaandConf(&conf)
	assert.NoError(t, err)

	err = build.Execute()
	assert.NoError(t, err)

	value1, _ := GetKey("maand/job/a/worker/10.0.0.1", "certs/a.crt")
	assert.NotEqual(t, value1, value2)
}

func TestMakefileNotFound(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"], "certs":{"a":{"subject":{"common":"fsadfsad"}}}}`), os.ModePerm)

	err = build.Execute()

	assert.Error(t, err)
	assert.ErrorContains(t, err, "invaild job: job a, Makefile not found")
}

func TestInvalidmanifestJSON(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", "a")
	_ = os.MkdirAll(jobPath, os.ModePerm)
	_ = os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(`{"selectors":["worker"],`), os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "jobs", "a", "Makefile"), nil, os.ModePerm)

	err = build.Execute()

	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid manifest.json: job a\nunexpected end of JSON input")
}
