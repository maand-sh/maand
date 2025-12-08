package tests

import (
	"io/fs"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
)

func TestBuild1(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.RemoveAll(path.Join(bucket.WorkspaceLocation, "workers.json"))

	err = build.Execute()
	assert.NoError(t, err)
}

func TestBuild2(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(``), fs.ModePerm)

	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrInvaildWorkerJSON)
}

func TestBuild3(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[]`), fs.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)
}

func TestBuild4(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{}]`), fs.ModePerm)

	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrInvaildWorkerJSON)
}

func TestBuild5(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`{}`), fs.ModePerm)

	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrInvaildWorkerJSON)
}

func TestBuild6(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"},{"host":"10.0.0.1"}]`), fs.ModePerm)

	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrInvaildWorkerJSON)
}

func TestBuild7(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1","labels":[1]}]`), fs.ModePerm)

	err = build.Execute()
	assert.ErrorIs(t, err, bucket.ErrInvaildWorkerJSON)
}

func TestBuild8(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), fs.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	namespace := "maand/worker/10.0.0.1"

	value, err := GetKey(namespace, "labels")
	assert.NoError(t, err)
	assert.Equal(t, "worker", value)

	value, err = GetKey(namespace, "worker_allocation_index")
	assert.NoError(t, err)
	assert.Equal(t, "0", value)

	value, err = GetKey(namespace, "worker_cpu_mhz")
	assert.NoError(t, err)
	assert.Equal(t, "0", value)

	value, err = GetKey(namespace, "worker_memory_mb")
	assert.NoError(t, err)
	assert.Equal(t, "0", value)

	value, err = GetKey(namespace, "worker_id")
	assert.NoError(t, err)
	assert.NotEmpty(t, value)

	value, err = GetKey(namespace, "worker_ip")
	assert.NoError(t, err)
	assert.Equal(t, "10.0.0.1", value)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"},{"host":"10.0.0.2"}]`), fs.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	namespace = "maand/worker/10.0.0.2"

	value, err = GetKey(namespace, "labels")
	assert.NoError(t, err)
	assert.Equal(t, "worker", value)

	value, err = GetKey(namespace, "worker_allocation_index")
	assert.NoError(t, err)
	assert.Equal(t, "1", value)

	value, err = GetKey(namespace, "worker_cpu_mhz")
	assert.NoError(t, err)
	assert.Equal(t, "0", value)

	value, err = GetKey(namespace, "worker_memory_mb")
	assert.NoError(t, err)
	assert.Equal(t, "0", value)

	value, err = GetKey(namespace, "worker_id")
	assert.NoError(t, err)
	assert.NotEmpty(t, value)

	value, err = GetKey(namespace, "worker_ip")
	assert.NoError(t, err)
	assert.Equal(t, "10.0.0.2", value)
}
