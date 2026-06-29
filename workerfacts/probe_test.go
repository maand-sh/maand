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

func TestParseProbeOutput(t *testing.T) {
	output, err := parseProbeOutput("MEMORY_MB=8192\nCPU_MHZ=9600\n")
	require.NoError(t, err)
	assert.Equal(t, 8192.0, output.MemoryMB)
	assert.Equal(t, 9600.0, output.CPUMHz)
}

func TestParseProbeOutputMissingFields(t *testing.T) {
	_, err := parseProbeOutput("CPU_MHZ=2400\n")
	assert.Error(t, err)

	_, err = parseProbeOutput("MEMORY_MB=0\nCPU_MHZ=2400\n")
	assert.Error(t, err)
}

func TestPreviewChanges(t *testing.T) {
	changes := previewChanges(
		[]workspace.WorkerRecord{
			{Host: "10.0.0.1", Memory: "1024 mb", CPU: "2000 mhz"},
		},
		map[string]workspace.WorkerFacts{
			"10.0.0.1": {MemoryMB: 8192, CPUMHz: 9600},
		},
	)
	assert.Len(t, changes, 1)
	assert.Equal(t, "8192 mb", changes[0].NewMemory)
}
