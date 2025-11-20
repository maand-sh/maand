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

// TestWorkerTag tests worker tags added, updated and removed
func TestWorkerTag(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{"host": "10.0.0.1", "tags": {"a":"v", "b": "v2"} }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	query := "SELECT group_concat(key), group_concat(value) FROM worker_tags"
	var keys, values string
	GetRowValues(query, &keys, &values)
	assert.Contains(t, strings.Split(keys, ","), "a")
	assert.Contains(t, strings.Split(keys, ","), "b")
	assert.Contains(t, strings.Split(values, ","), "v")
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

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[
		{"host": "10.0.0.1", "tags": {"a":"v1", "b": "v2", "c": "v3"} }
	]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	GetRowValues(query, &keys, &values)
	assert.Contains(t, strings.Split(keys, ","), "a")
	assert.Contains(t, strings.Split(keys, ","), "b")
	assert.Contains(t, strings.Split(keys, ","), "c")

	assert.Contains(t, strings.Split(values, ","), "v1")
	assert.Contains(t, strings.Split(values, ","), "v2")
	assert.Contains(t, strings.Split(values, ","), "v3")

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
