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

func TestWorkerResources(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	var memory, cpu string
	GetRowValues("SELECT available_memory_mb, available_cpu_mhz FROM worker", &memory, &cpu)
	assert.Equal(t, "0", memory)
	assert.Equal(t, "0", cpu)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "memory": "23", "cpu": "100" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues("SELECT available_memory_mb, available_cpu_mhz FROM worker", &memory, &cpu)
	assert.Equal(t, "23", memory)
	assert.Equal(t, "100", cpu)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "memory": "23 GB", "cpu": "100 GHz" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues("SELECT available_memory_mb, available_cpu_mhz FROM worker", &memory, &cpu)
	assert.Equal(t, "23552", memory)
	assert.Equal(t, "100000", cpu)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues("SELECT available_memory_mb, available_cpu_mhz FROM worker", &memory, &cpu)
	assert.Equal(t, "0", memory)
	assert.Equal(t, "0", cpu)
}

func TestWorkerResourcesInvalid(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "cpu": "-1" }
	]`), os.ModePerm)

	assert.ErrorIs(t, build.Execute(), build.ErrInvaildWorkerJSON)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "memory": "-1" }
	]`), os.ModePerm)

	assert.ErrorIs(t, build.Execute(), build.ErrInvaildWorkerJSON)
}
