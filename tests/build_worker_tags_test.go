package tests

import (
	"os"
	"path"
	"strings"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
)

// TestWorkerTagAdded tests worker tags added
func TestWorkerTagAdded(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{"host": "10.0.0.1" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	query := "SELECT group_concat(key), group_concat(value) FROM worker_tags"
	var keys, values string

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{"host": "10.0.0.1", "tags": {"a":"v", "b": "v3"} }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues(query, &keys, &values)
	assert.Contains(t, strings.Split(keys, ","), "a")
	assert.Contains(t, strings.Split(keys, ","), "b")
	assert.Contains(t, strings.Split(values, ","), "v")
	assert.Contains(t, strings.Split(values, ","), "v3")
}

// TestWorkerTagUpdated tests worker tags updated
func TestWorkerTagUpdated(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{"host": "10.0.0.1", "tags": {"b": "v2"} }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	query := "SELECT group_concat(key), group_concat(value) FROM worker_tags"
	var keys, values string
	GetRowValues(query, &keys, &values)
	assert.Contains(t, strings.Split(keys, ","), "b")
	assert.Contains(t, strings.Split(values, ","), "v2")

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{"host": "10.0.0.1", "tags": {"a":"v", "b": "v3"} }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues(query, &keys, &values)
	assert.Contains(t, strings.Split(keys, ","), "a")
	assert.Contains(t, strings.Split(keys, ","), "b")
	assert.Contains(t, strings.Split(values, ","), "v")
	assert.Contains(t, strings.Split(values, ","), "v3")
}

// TestWorkerTagRemoved tests worker tags removed
func TestWorkerTagRemoved(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{"host": "10.0.0.1", "tags": {"a":"v", "b": "v2"} }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	query := "SELECT ifnull(group_concat(key), ''), ifnull(group_concat(value), '') FROM worker_tags"
	var keys, values string

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{"host": "10.0.0.1" }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	values = ""
	keys = ""
	GetRowValues(query, &keys, &values)
	assert.Empty(t, "", keys)
	assert.Equal(t, "", values)
}
