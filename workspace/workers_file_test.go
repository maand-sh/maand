// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyWorkerFactsUpdatesWorkersJSON(t *testing.T) {
	root := t.TempDir()
	bucket.Location = root
	bucket.WorkspaceLocation = path.Join(root, "workspace")
	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(workersJSONPath(), []byte(`[
  {"host":"10.0.0.1","labels":["prod"],"memory":"1024 mb","cpu":"2000 mhz"},
  {"host":"10.0.0.2","memory":"512 mb","cpu":"1000 mhz"}
]`), 0o644))

	changes, err := ApplyWorkerFacts(map[string]WorkerFacts{
		"10.0.0.1": {MemoryMB: 8192, CPUMHz: 9600},
	})
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "8192 mb", changes[0].NewMemory)
	assert.Equal(t, "9600 mhz", changes[0].NewCPU)

	raw, err := os.ReadFile(workersJSONPath())
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"memory": "8192 mb"`)
	assert.Contains(t, string(raw), `"cpu": "9600 mhz"`)
	assert.Contains(t, string(raw), `"memory": "512 mb"`)
}

func TestApplyWorkerFactsSkipsUnchangedValues(t *testing.T) {
	root := t.TempDir()
	bucket.Location = root
	bucket.WorkspaceLocation = path.Join(root, "workspace")
	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(workersJSONPath(), []byte(`[
  {"host":"10.0.0.1","memory":"4096 mb","cpu":"4000 mhz"}
]`), 0o644))

	changes, err := ApplyWorkerFacts(map[string]WorkerFacts{
		"10.0.0.1": {MemoryMB: 4096, CPUMHz: 4000},
	})
	require.NoError(t, err)
	assert.Empty(t, changes)
}

func TestWriteWorkersFileOmitsNullFields(t *testing.T) {
	root := t.TempDir()
	bucket.Location = root
	bucket.WorkspaceLocation = path.Join(root, "workspace")
	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))

	workers := []WorkerRecord{
		{Host: "10.0.0.1", Memory: "8192 mb", CPU: "9600 mhz"},
	}
	require.NoError(t, WriteWorkersFile(workers))

	raw, err := os.ReadFile(workersJSONPath())
	require.NoError(t, err)
	body := string(raw)
	assert.NotContains(t, body, "null")
	assert.Contains(t, body, `"host": "10.0.0.1"`)
	assert.Contains(t, body, `"memory": "8192 mb"`)
	assert.Contains(t, body, `"cpu": "9600 mhz"`)
}

func TestWriteWorkersFileOmitsNullFieldsAfterRead(t *testing.T) {
	root := t.TempDir()
	bucket.Location = root
	bucket.WorkspaceLocation = path.Join(root, "workspace")
	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(workersJSONPath(), []byte(`[
  {"host":"10.0.0.1","labels":null,"memory":"1024 mb","cpu":"2000 mhz","tags":null,"position":null}
]`), 0o644))

	workers, err := ReadWorkersFile()
	require.NoError(t, err)
	require.NoError(t, WriteWorkersFile(workers))

	raw, err := os.ReadFile(workersJSONPath())
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "null")
}

func TestApplyWorkerFactsPreservesExistingFields(t *testing.T) {
	root := t.TempDir()
	bucket.Location = root
	bucket.WorkspaceLocation = path.Join(root, "workspace")
	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(workersJSONPath(), []byte(`[
  {"host":"10.0.0.1","labels":["prod"],"tags":{"zone":"a"},"memory":"1024 mb","cpu":"2000 mhz"}
]`), 0o644))

	_, err := ApplyWorkerFacts(map[string]WorkerFacts{
		"10.0.0.1": {MemoryMB: 8192, CPUMHz: 9600},
	})
	require.NoError(t, err)

	raw, err := os.ReadFile(workersJSONPath())
	require.NoError(t, err)
	body := string(raw)
	assert.NotContains(t, body, "null")
	assert.Contains(t, body, `"labels"`)
	assert.Contains(t, body, `"prod"`)
	assert.Contains(t, body, `"tags"`)
	assert.Contains(t, body, `"zone"`)
	assert.NotContains(t, body, `"position"`)
}

func TestFilterWorkersByLabels(t *testing.T) {
	workers := []WorkerRecord{
		{Host: "10.0.0.1", Labels: []string{"prod"}},
		{Host: "10.0.0.2", Labels: []string{"staging"}},
	}

	filtered := FilterWorkersByLabels(workers, []string{"prod"})
	require.Len(t, filtered, 1)
	assert.Equal(t, "10.0.0.1", filtered[0].Host)
}
