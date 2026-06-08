// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"testing"

	"maand/bucket"
	"maand/worker"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWorkerCommandUsesHooks(t *testing.T) {
	called := false
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, workerIP string, commands []string, _ []string) error {
			called = true
			assert.Equal(t, "10.0.0.1", workerIP)
			assert.Len(t, commands, 1)
			return nil
		},
	})
	t.Cleanup(ClearTestHooks)

	require.NoError(t, runWorkerCommand(nil, "10.0.0.1", []string{"echo ok"}, nil))
	assert.True(t, called)
}

func TestRunRsyncUsesHooks(t *testing.T) {
	called := false
	SetTestHooks(&TestHooks{
		Rsync: func(_ *bucket.Runtime, bucketID, workerIP string) error {
			called = true
			assert.Equal(t, "bucket-1", bucketID)
			assert.Equal(t, "10.0.0.1", workerIP)
			return nil
		},
	})
	t.Cleanup(ClearTestHooks)

	require.NoError(t, runRsync(nil, "bucket-1", "10.0.0.1"))
	assert.True(t, called)
}

func TestSetupDeployRuntimeUsesHooks(t *testing.T) {
	SetTestHooks(&TestHooks{
		SetupRuntime: func(bucketID string) (*bucket.Runtime, error) {
			assert.Equal(t, "bucket-1", bucketID)
			return &bucket.Runtime{}, nil
		},
	})
	t.Cleanup(ClearTestHooks)

	rt, err := setupDeployRuntime("bucket-1")
	require.NoError(t, err)
	require.NotNil(t, rt)
}

func TestRunWorkerCommandWithoutDeployHooks(t *testing.T) {
	ClearTestHooks()
	worker.SetTestHooks(&worker.TestHooks{
		ExecuteCommand: func(_ *bucket.Runtime, _ string, _ []string, _ []string) error {
			return nil
		},
	})
	t.Cleanup(func() {
		worker.ClearTestHooks()
		ClearTestHooks()
	})

	require.NoError(t, runWorkerCommand(nil, "10.0.0.1", []string{"echo ok"}, nil))
}
