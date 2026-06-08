// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveJobDeployArtifactsFromWorker_live(t *testing.T) {
	env := setupDeployTestEnv(t)
	var ran bool
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, workerIP string, commands []string, _ []string) error {
			assert.Equal(t, "10.0.0.1", workerIP)
			ran = len(commands) == 1
			return nil
		},
	})
	t.Cleanup(ClearTestHooks)

	workerDir := bucket.GetTempWorkerPath("10.0.0.1")
	jobDir := path.Join(workerDir, "jobs", "app")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "marker.txt"), []byte("x"), 0o644))

	require.NoError(t, removeJobDeployArtifactsFromWorker(nil, env.bucketID, "10.0.0.1", "app", false))
	assert.True(t, ran)
	_, err := os.Stat(jobDir)
	assert.True(t, os.IsNotExist(err))
}

func TestRemoveJobDeployArtifactsFromWorker_assumeDead(t *testing.T) {
	env := setupDeployTestEnv(t)
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, _ string, _ []string, _ []string) error {
			return assert.AnError
		},
	})
	t.Cleanup(ClearTestHooks)

	workerDir := bucket.GetTempWorkerPath("10.0.0.1")
	require.NoError(t, os.MkdirAll(path.Join(workerDir, "jobs", "app"), 0o755))

	require.NoError(t, removeJobDeployArtifactsFromWorker(nil, env.bucketID, "10.0.0.1", "app", true))
	_, err := os.Stat(path.Join(workerDir, "jobs", "app"))
	assert.True(t, os.IsNotExist(err))
}

func TestRemoveWorkerBucketFromWorker(t *testing.T) {
	env := setupDeployTestEnv(t)
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, _ string, _ []string, _ []string) error {
			return nil
		},
	})
	t.Cleanup(ClearTestHooks)

	workerDir := bucket.GetTempWorkerPath("10.0.0.2")
	require.NoError(t, os.MkdirAll(workerDir, 0o755))
	require.NoError(t, removeWorkerBucketFromWorker(nil, env.bucketID, "10.0.0.2"))

	_, err := os.Stat(workerDir)
	assert.True(t, os.IsNotExist(err))
}
