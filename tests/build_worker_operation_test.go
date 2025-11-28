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

// valid worker json
// A worker get added later
func TestWorkerOperationAdded(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)
	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1" },
		{ "host": "10.0.0.2" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT COUNT(*) FROM worker")
	assert.Equal(t, count, 2)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1" },
		{ "host": "10.0.0.2" },
		{ "host": "10.0.0.3" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT COUNT(*) FROM worker")
	assert.Equal(t, count, 3)

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

// A worker get updated
func TestWorkerOperationUpdated(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)
	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1" },
		{ "host": "10.0.0.2" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT COUNT(*) FROM worker")
	assert.Equal(t, count, 2)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1" },
		{ "host": "10.0.0.3" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT COUNT(*) FROM worker")
	assert.Equal(t, count, 2)

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

// A worker get removed later
func TestWorkerOperationRemoved(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1" },
		{ "host": "10.0.0.2" },
		{ "host": "10.0.0.3" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count := GetRowCount("SELECT COUNT(*) FROM worker")
	assert.Equal(t, count, 3)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1" },
		{ "host": "10.0.0.3" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	count = GetRowCount("SELECT COUNT(*) FROM worker")
	assert.Equal(t, count, 2)

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

func TestWorkerDefaultKVLabels(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)
	err := initialize.Execute()
	assert.NoError(t, err)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{"host": "10.0.0.1"},
		{"host": "10.0.0.3"}
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	err = cat.KV()
	assert.NoError(t, err)

	value, _ := GetKey("maand/worker", "worker_workers")
	assert.Equal(t, "10.0.0.1,10.0.0.3", value)

	value, _ = GetKey("maand/worker", "worker_workers_length")
	assert.Equal(t, "2", value)

	value, _ = GetKey("maand/worker", "worker_label_id")
	assert.NotEmpty(t, value)

	value, _ = GetKey("maand/worker", "worker_0")
	assert.Equal(t, "10.0.0.1", value)
	value, _ = GetKey("maand/worker", "worker_1")
	assert.Equal(t, "10.0.0.3", value)

	value, _ = GetKey("maand/worker/10.0.0.1", "labels")
	assert.Equal(t, "worker", value)
	value, _ = GetKey("maand/worker/10.0.0.3", "labels")
	assert.Equal(t, "worker", value)

	value, _ = GetKey("maand/worker/10.0.0.1", "worker_allocation_index")
	assert.Equal(t, "0", value)
	value, _ = GetKey("maand/worker/10.0.0.3", "worker_allocation_index")
	assert.Equal(t, "1", value)

	worker1ID, _ := GetKey("maand/worker/10.0.0.1", "worker_id")
	assert.NotEmpty(t, value)
	worker2ID, _ := GetKey("maand/worker/10.0.0.3", "worker_id")
	assert.NotEmpty(t, value)
	assert.NotEqual(t, worker2ID, worker1ID)

	value, _ = GetKey("maand/worker/10.0.0.1", "worker_ip")
	assert.Equal(t, "10.0.0.1", value)
	value, _ = GetKey("maand/worker/10.0.0.3", "worker_ip")
	assert.Equal(t, "10.0.0.3", value)

	value, _ = GetKey("maand/worker/10.0.0.1", "worker_peers")
	assert.Equal(t, "10.0.0.3", value)
	value, _ = GetKey("maand/worker/10.0.0.3", "worker_peers")
	assert.Equal(t, "10.0.0.1", value)

	value, _ = GetKey("maand/worker/10.0.0.1", "worker_memory_mb")
	assert.Equal(t, "0", value)
	value, _ = GetKey("maand/worker/10.0.0.3", "worker_memory_mb")
	assert.Equal(t, "0", value)

	value, _ = GetKey("maand/worker/10.0.0.1", "worker_cpu_mhz")
	assert.Equal(t, "0", value)
	value, _ = GetKey("maand/worker/10.0.0.3", "worker_cpu_mhz")
	assert.Equal(t, "0", value)
}
