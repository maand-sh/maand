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
	"maand/data"
	"maand/deploy"
	"maand/runcommand"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationEndToEndBuildTarget verifies build syncs catalog/KV without allocation
// plan hashes, and deploy owns staging, hash refresh, rollout, and promotion.
func TestIntegrationEndToEndBuildTarget(t *testing.T) {
	setupIntegrationWorkspace(t)

	executeBuild(t)

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	jobs, err := data.GetJobs(tx)
	require.NoError(t, err)
	require.Contains(t, jobs, integrationJobName)

	active, err := data.CountAllocations(tx, true)
	require.NoError(t, err)
	require.Equal(t, len(workerIPs(t)), active)

	assert.Equal(t, 0, countJobAllocationHashes(t, integrationJobName),
		"build must not write job allocation plan hashes")

	result, err := deploy.DryRun(nil, false)
	require.NoError(t, err)
	assert.True(t, result.Required, "first deploy should be required after build only")

	require.NoError(t, deploy.Execute(nil, false))
	assert.Greater(t, countJobAllocationHashes(t, integrationJobName), 0)
	assert.True(t, jobAllocationHashesPromoted(t, integrationJobName))

	result, err = deploy.DryRun(nil, false)
	require.NoError(t, err)
	assert.False(t, result.Required, "deploy should be in sync after promote")

	executeBuild(t)
	result, err = deploy.DryRun(nil, false)
	require.NoError(t, err)
	assert.False(t, result.Required, "unchanged rebuild should not require deploy")

	marker := filepath.Join(bucket.WorkspaceLocation, "jobs", integrationJobName, "marker.txt")
	require.NoError(t, os.WriteFile(marker, []byte("integration-v1"), 0o644))
	executeBuild(t)

	assert.Greater(t, countJobAllocationHashes(t, integrationJobName), 0,
		"rebuild must not remove existing allocation hash rows")

	result, err = deploy.DryRun(nil, false)
	require.NoError(t, err)
	assert.True(t, result.Required, "deploy plan-hash refresh should detect workspace change after rebuild")

	require.NoError(t, deploy.Execute(nil, false))
	assert.True(t, jobAllocationHashesPromoted(t, integrationJobName))

	result, err = deploy.DryRun(nil, false)
	require.NoError(t, err)
	assert.False(t, result.Required)

	bucketID, err := data.GetBucketID(tx)
	require.NoError(t, err)
	cmd := "cat /opt/worker/" + bucketID + "/jobs/" + integrationJobName + "/marker.txt"
	require.NoError(t, runcommand.Execute(workerIPs(t)[0], "", 1, cmd, false))
}
