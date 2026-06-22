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
	"maand/deploy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationDeployVersionFirstDeploy(t *testing.T) {
	setupVersionIntegrationBucket(t, "1.0.0")
	assert.Equal(t, "1.0.0", jobCatalogVersion(t, integrationJobName))
	assert.Equal(t, "1.0.0", latestKVValue(t, "maand/job/"+integrationJobName, "version"))

	require.NoError(t, deploy.Execute(nil, false))
	assert.True(t, jobAllocationHashesPromoted(t, integrationJobName))
	assert.True(t, jobAllocationVersionsPromoted(t, integrationJobName))

	ip := workerIPs(t)[0]
	current, newVer := allocationHashVersions(t, integrationJobName, ip)
	assert.Equal(t, "1.0.0", current)
	assert.Equal(t, "1.0.0", newVer)

	assert.Equal(t, "0.0.0", workerVersionData(t, ip, "current_version"))
	assert.Equal(t, "1.0.0", workerVersionData(t, ip, "new_version"))
	assert.Equal(t, "1.0.0", latestKVValue(t, "maand/job/"+integrationJobName+"/worker/"+ip, "version"))
}

func TestIntegrationDeployVersionOmittedDefaultsToZero(t *testing.T) {
	setupVersionIntegrationBucket(t, "")
	assert.Equal(t, "unknown", jobCatalogVersion(t, integrationJobName))
	assert.Equal(t, "0.0.0", latestKVValue(t, "maand/job/"+integrationJobName, "version"))

	require.NoError(t, deploy.Execute(nil, false))
	assert.True(t, jobAllocationVersionsPromoted(t, integrationJobName))

	ip := workerIPs(t)[0]
	current, newVer := allocationHashVersions(t, integrationJobName, ip)
	assert.Equal(t, "0.0.0", current)
	assert.Equal(t, "0.0.0", newVer)
}

func TestIntegrationDeployVersionUpgrade(t *testing.T) {
	setupVersionIntegrationBucket(t, "1.0.0")
	require.NoError(t, deploy.Execute(nil, false))
	assert.True(t, jobAllocationVersionsPromoted(t, integrationJobName))

	ip := workerIPs(t)[0]
	writeVersionedIntegrationJob(t, "2.0.0")
	executeBuild(t)
	assert.Equal(t, "2.0.0", latestKVValue(t, "maand/job/"+integrationJobName, "version"))

	plan := dryRunPlanForJob(t, integrationJobName)
	assert.True(t, plan.NeedsRollout)
	assert.Equal(t, "restart", plan.Allocations[0].Action)

	require.NoError(t, deploy.Execute(nil, false))
	assert.True(t, jobAllocationVersionsPromoted(t, integrationJobName))
	assert.Equal(t, 1, workerJobCounter(t, ip, "restart"))

	current, newVer := allocationHashVersions(t, integrationJobName, ip)
	assert.Equal(t, "2.0.0", current)
	assert.Equal(t, "2.0.0", newVer)

	assert.Equal(t, "1.0.0", workerVersionData(t, ip, "current_version"))
	assert.Equal(t, "2.0.0", workerVersionData(t, ip, "new_version"))
	assert.Equal(t, "2.0.0", latestKVValue(t, "maand/job/"+integrationJobName+"/worker/"+ip, "version"))

	result, err := deploy.DryRun(nil, false)
	require.NoError(t, err)
	assert.False(t, result.Required)
}

func TestIntegrationDeployVersionSameVersionNoRollout(t *testing.T) {
	setupVersionIntegrationBucket(t, "1.0.0")
	require.NoError(t, deploy.Execute(nil, false))

	result, err := deploy.DryRun(nil, false)
	require.NoError(t, err)
	assert.False(t, result.Required)

	require.NoError(t, deploy.Execute(nil, false))

	ip := workerIPs(t)[0]
	assert.Equal(t, 1, workerJobCounter(t, ip, "start"))
	assert.Equal(t, 0, workerJobCounter(t, ip, "restart"))
}

func TestIntegrationDeployVersionUpgradeWithoutManifestVersionChange(t *testing.T) {
	setupVersionIntegrationBucket(t, "1.0.0")
	require.NoError(t, deploy.Execute(nil, false))

	writeVersionedIntegrationJob(t, "1.0.0")
	require.NoError(t, os.WriteFile(
		filepath.Join(bucket.WorkspaceLocation, "jobs", integrationJobName, "upgrade-marker.txt"),
		[]byte("content-only"),
		0o644,
	))
	executeBuild(t)

	plan := dryRunPlanForJob(t, integrationJobName)
	assert.True(t, plan.NeedsRollout)

	ip := workerIPs(t)[0]
	current, newVer := allocationHashVersions(t, integrationJobName, ip)
	assert.Equal(t, "1.0.0", current)
	assert.Equal(t, "1.0.0", newVer)

	require.NoError(t, deploy.Execute(nil, false))
	assert.Equal(t, "1.0.0", workerVersionData(t, ip, "current_version"))
	assert.Equal(t, "1.0.0", workerVersionData(t, ip, "new_version"))
}
