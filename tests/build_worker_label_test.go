package tests

import (
	"fmt"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
)

func TestWorkerLabelDefault(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" }]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	var labels string

	query := "SELECT GROUP_CONCAT(label) FROM (SELECT label FROM worker_labels WHERE worker_id = (select worker_id FROM worker WHERE worker_ip = '%s') ORDER BY label)"
	GetRowValues(fmt.Sprintf(query, "10.0.0.1"), &labels)
	assert.Equal(t, labels, "worker")
}

func TestWorkerLabelsAdded(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)
	err := initialize.Execute()
	assert.NoError(t, err)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "labels": ["a", "z"] },
		{ "host": "10.0.0.2", "labels": ["b"] }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	var labels string

	query := "SELECT GROUP_CONCAT(label) FROM (SELECT label FROM worker_labels WHERE worker_id = (select worker_id FROM worker WHERE worker_ip = '%s') ORDER BY label)"
	GetRowValues(fmt.Sprintf(query, "10.0.0.1"), &labels)
	assert.Equal(t, "a,worker,z", labels)

	GetRowValues(fmt.Sprintf(query, "10.0.0.2"), &labels)
	assert.Equal(t, "b,worker", labels)
}

func TestWorkerLabelsRemoved(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)
	err := initialize.Execute()
	assert.NoError(t, err)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "labels": ["a", "z"] },
		{ "host": "10.0.0.2" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	var labels string

	query := "SELECT GROUP_CONCAT(label) FROM (SELECT label FROM worker_labels WHERE worker_id = (select worker_id FROM worker WHERE worker_ip = '%s') ORDER BY label)"
	GetRowValues(fmt.Sprintf(query, "10.0.0.1"), &labels)
	assert.Equal(t, labels, "a,worker,z")

	GetRowValues(fmt.Sprintf(query, "10.0.0.2"), &labels)
	assert.Equal(t, labels, "worker")

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "labels": ["a"] },
		{ "host": "10.0.0.2" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	query = "SELECT GROUP_CONCAT(label) FROM (SELECT label FROM worker_labels WHERE worker_id = (select worker_id FROM worker WHERE worker_ip = '%s') ORDER BY label)"
	GetRowValues(fmt.Sprintf(query, "10.0.0.1"), &labels)
	assert.Equal(t, labels, "a,worker")
}

func TestWorkerLabelsUpdated(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "labels": ["a", "z"] }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	var labels string

	query := "SELECT GROUP_CONCAT(label) FROM (SELECT label FROM worker_labels WHERE worker_id = (select worker_id FROM worker WHERE worker_ip = '%s') ORDER BY label)"
	GetRowValues(fmt.Sprintf(query, "10.0.0.1"), &labels)
	assert.Equal(t, labels, "a,worker,z")

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{ "host": "10.0.0.1", "labels": ["a", "v"] }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues(fmt.Sprintf(query, "10.0.0.1"), &labels)
	assert.Equal(t, labels, "a,v,worker")
}

func TestWorkerLabelsDuplicated(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1", "labels": ["a", "a"] }]`), os.ModePerm)

	assert.ErrorIs(t, build.Execute(), build.ErrInvaildWorkerJSON)
}
