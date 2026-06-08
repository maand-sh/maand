// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinishRemovedWorkerCommand_nilError(t *testing.T) {
	require.NoError(t, finishRemovedWorkerCommand("10.0.0.1", nil, false))
}

func TestFinishRemovedWorkerCommand_assumeDead(t *testing.T) {
	require.NoError(t, finishRemovedWorkerCommand("10.0.0.1", assert.AnError, true))
}

func TestFinishRemovedWorkerCommand_propagates(t *testing.T) {
	err := finishRemovedWorkerCommand("10.0.0.1", assert.AnError, false)
	require.Error(t, err)
}

func TestRunWorkerCommandOrAssumeDead_swallowsError(t *testing.T) {
	ClearTestHooks()
	t.Cleanup(ClearTestHooks)

	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, _ string, _ []string, _ []string) error {
			return assert.AnError
		},
	})

	runWorkerCommandOrAssumeDead(nil, "10.0.0.99", []string{"echo test"}, nil)
}
