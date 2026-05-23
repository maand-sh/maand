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

func TestWorkerJSONEmpty(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	err = build.Execute()
	assert.NoError(t, err)
}

func TestWorkerJSONValid(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.Remove(path.Join(bucket.WorkspaceLocation, "workers.json"))
	err = build.Execute()

	assert.NoError(t, err)
}

func TestWorkerJSON3Worker(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.2" }]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := MustQueryCount(t, "SELECT COUNT(*) FROM worker")
	assert.Equal(t, 2, count)
}

func TestWorkerJSONDuplicateWorker(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.1" }]`), os.ModePerm)
	err = build.Execute()

	assert.ErrorIs(t, err, bucket.ErrInvalidWorkerJSON)
}

func TestWorkerJSONInvalid(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`{}`), os.ModePerm)
	err = build.Execute()

	assert.ErrorIs(t, err, bucket.ErrInvalidWorkerJSON)
}

func TestWorkerJSONRemains(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	_ = os.MkdirAll(bucket.WorkspaceLocation, os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" }]`), os.ModePerm)

	err := initialize.Execute()
	assert.NoError(t, err)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT count(1) FROM worker")
	assert.Equal(t, 1, count)
}
