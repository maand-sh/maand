// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workerfacts

import (
	"testing"

	"maand/workspace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectTargetWorkersByIP(t *testing.T) {
	workers := []workspace.WorkerRecord{
		{Host: "10.0.0.1"},
		{Host: "10.0.0.2"},
	}

	selected, err := selectTargetWorkers(workers, "10.0.0.2", "")
	require.NoError(t, err)
	require.Len(t, selected, 1)
	assert.Equal(t, "10.0.0.2", selected[0].Host)
}

func TestSelectTargetWorkersUnknownIP(t *testing.T) {
	workers := []workspace.WorkerRecord{{Host: "10.0.0.1"}}

	_, err := selectTargetWorkers(workers, "10.0.0.9", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10.0.0.9")
}

func TestSelectTargetWorkersByLabel(t *testing.T) {
	workers := []workspace.WorkerRecord{
		{Host: "10.0.0.1", Labels: []string{"prod"}},
		{Host: "10.0.0.2", Labels: []string{"staging"}},
	}

	selected, err := selectTargetWorkers(workers, "", "prod")
	require.NoError(t, err)
	require.Len(t, selected, 1)
	assert.Equal(t, "10.0.0.1", selected[0].Host)
}
