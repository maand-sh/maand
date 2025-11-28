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

func TestWorkerCPU(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	var cpu string
	query := "SELECT available_cpu_mhz FROM worker"
	GetRowValues(query, &cpu)
	assert.Equal(t, "0", cpu)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "cpu": "100" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues(query, &cpu)
	assert.Equal(t, "100", cpu)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "cpu": "100 GHz" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues(query, &cpu)
	assert.Equal(t, "100000", cpu)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues(query, &cpu)
	assert.Equal(t, "0", cpu)
}

func TestWorkerMemory(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	var memory string
	query := "SELECT available_memory_mb  FROM worker WHERE worker_ip = '10.0.0.1'"
	GetRowValues(query, &memory)
	assert.Equal(t, "0", memory)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "memory": "23" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues(query, &memory)
	assert.Equal(t, "23", memory)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "memory": "23 GB" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues(query, &memory)
	assert.Equal(t, "23552", memory)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues(query, &memory)
	assert.Equal(t, "0", memory)
}

func TestWorkerResourcesInvalid(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "cpu": "-1" }
	]`), os.ModePerm)

	assert.ErrorIs(t, build.Execute(), bucket.ErrInvaildWorkerJSON)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "memory": "-1" }
	]`), os.ModePerm)

	assert.ErrorIs(t, build.Execute(), bucket.ErrInvaildWorkerJSON)
}

func TestWorkerResourcesKVUpdated(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "memory": "10", "cpu": "100" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ := GetKey("maand/worker/10.0.0.1", "worker_memory_mb")
	assert.Equal(t, "10", value)
	value, _ = GetKey("maand/worker/10.0.0.1", "worker_cpu_mhz")
	assert.Equal(t, "100", value)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "memory": "20", "cpu": "200" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	value, _ = GetKey("maand/worker/10.0.0.1", "worker_memory_mb")
	assert.Equal(t, "20", value)
	value, _ = GetKey("maand/worker/10.0.0.1", "worker_cpu_mhz")
	assert.Equal(t, "200", value)
}
