package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
)

// test workers position
// added
func TestWorkerPostionAdded(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.2" }]`), os.ModePerm)

	err = executeBuildErr(t)
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.2" },{ "host": "10.0.0.3" }]`), os.ModePerm)

	err = executeBuildErr(t)
	assert.NoError(t, err)

	assert.Equal(t, map[string]int{"10.0.0.1": 0, "10.0.0.2": 1, "10.0.0.3": 2}, scanWorkerPositions(t))
}

// moved
func TestWorkerPostionMoved(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.3" }]`), os.ModePerm)

	err = executeBuildErr(t)
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.2" },{ "host": "10.0.0.3" }]`), os.ModePerm)

	err = executeBuildErr(t)
	assert.NoError(t, err)

	assert.Equal(t, map[string]int{"10.0.0.1": 0, "10.0.0.2": 1, "10.0.0.3": 2}, scanWorkerPositions(t))
}

// removed
func TestWorkerPostionRemoved(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.2" },{ "host": "10.0.0.3" }]`), os.ModePerm)

	err = executeBuildErr(t)
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.3" }]`), os.ModePerm)

	err = executeBuildErr(t)
	assert.NoError(t, err)

	assert.Equal(t, map[string]int{"10.0.0.1": 0, "10.0.0.3": 1}, scanWorkerPositions(t))
}
