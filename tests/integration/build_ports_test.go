// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationJobPortsStableAcrossRebuild(t *testing.T) {
	setupPortOnlyIntegrationBucket(t)

	portName := "integapp_port_http"
	port1 := jobAssignedPort(t, integrationJobName, portName)
	kv1 := latestKVValue(t, "maand", portName)
	assert.Equal(t, port1, kv1)

	executeBuild(t)
	port2 := jobAssignedPort(t, integrationJobName, portName)
	kv2 := latestKVValue(t, "maand", portName)
	assert.Equal(t, port1, port2)
	assert.Equal(t, port1, kv2)

	jobDir := filepath.Join(bucket.WorkspaceLocation, "jobs", integrationJobName)
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "rebuild-marker"), []byte("touch"), 0o644))

	executeBuild(t)
	port3 := jobAssignedPort(t, integrationJobName, portName)
	kv3 := latestKVValue(t, "maand", portName)
	assert.Equal(t, port1, port3)
	assert.Equal(t, port1, kv3)
}
