// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"maand/build"
	"maand/bucket"
	"maand/deploy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationBuildAndDeploy(t *testing.T) {
	setupIntegrationBucket(t)

	require.NoError(t, deploy.Execute(nil, false))
}

func TestIntegrationDeployDryRunAfterPromote(t *testing.T) {
	setupIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil, false))

	result, err := deploy.DryRun(nil, false)
	require.NoError(t, err)
	assert.False(t, result.Required, "dry-run after successful deploy should not require rollout")
}

func TestIntegrationDeployRolloutAfterFileChange(t *testing.T) {
	setupIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil, false))

	result, err := deploy.DryRun(nil, false)
	require.NoError(t, err)
	assert.False(t, result.Required)

	marker := filepath.Join(bucket.WorkspaceLocation, "jobs", integrationJobName, "marker.txt")
	require.NoError(t, os.WriteFile(marker, []byte("changed"), 0o644))
	require.NoError(t, build.Execute())

	result, err = deploy.DryRun(nil, false)
	require.NoError(t, err)
	assert.True(t, result.Required, "dry-run should detect content change after rebuild")

	require.NoError(t, deploy.Execute(nil, false))

	result, err = deploy.DryRun(nil, false)
	require.NoError(t, err)
	assert.False(t, result.Required, "dry-run after rollout should show bucket in sync")
}

func TestIntegrationDeployJobFilter(t *testing.T) {
	setupIntegrationBucket(t)
	require.NoError(t, deploy.Execute([]string{integrationJobName}, false))
}
