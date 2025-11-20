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

// test workers position
// added
func TestBuildWorkerPostionAdded(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.2" }]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.2" },{ "host": "10.0.0.3" }]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	rows := GetRows("SELECT worker_ip, position FROM worker")
	workers := make(map[string]int)
	for rows.Next() {
		var workerIP string
		var position int
		_ = rows.Scan(&workerIP, &position)
		workers[workerIP] = position
	}

	e := map[string]int{"10.0.0.1": 0, "10.0.0.2": 1, "10.0.0.3": 2}
	assert.Equal(t, e, workers)
}

// moved
func TestBuildWorkerPostionMoved(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.3" }]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.2" },{ "host": "10.0.0.3" }]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	rows := GetRows("SELECT worker_ip, position FROM worker")
	workers := make(map[string]int)
	for rows.Next() {
		var workerIP string
		var position int
		_ = rows.Scan(&workerIP, &position)
		workers[workerIP] = position
	}

	e := map[string]int{"10.0.0.1": 0, "10.0.0.2": 1, "10.0.0.3": 2}
	assert.Equal(t, e, workers)
}

// removed
func TestBuildWorkerPostionRemoved(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.2" },{ "host": "10.0.0.3" }]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.3" }]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	rows := GetRows("SELECT worker_ip, position FROM worker")
	workers := make(map[string]int)
	for rows.Next() {
		var workerIP string
		var position int
		_ = rows.Scan(&workerIP, &position)
		workers[workerIP] = position
	}

	e := map[string]int{"10.0.0.1": 0, "10.0.0.3": 1}
	assert.Equal(t, e, workers)
}
