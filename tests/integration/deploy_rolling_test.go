// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"maand/bucket"
	"maand/deploy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationDeployRollingUpgradeOnContentChange(t *testing.T) {
	ips := workerIPsFromAssets(t)
	if len(ips) < 2 {
		t.Skip("rolling upgrade integration test requires at least two workers in assets/workers.json")
	}

	const parallel = 1
	setupRollingIntegrationBucket(t, parallel)
	assert.Equal(t, parallel, jobUpdateParallelCount(t, integrationJobName))

	require.NoError(t, deploy.Execute(nil, deploy.Options{}))
	assert.True(t, jobAllocationHashesPromoted(t, integrationJobName))

	for _, ip := range ips {
		assert.Equal(t, 1, workerJobCounter(t, ip, "start"), "first deploy should start on %s", ip)
		assert.Equal(t, 0, workerJobCounter(t, ip, "restart"), "first deploy should not restart on %s", ip)
	}

	marker := filepath.Join(bucket.WorkspaceLocation, "jobs", integrationJobName, "upgrade-marker.txt")
	require.NoError(t, os.WriteFile(marker, []byte("v2"), 0o644))
	executeBuild(t)

	plan := dryRunPlanForJob(t, integrationJobName)
	assert.True(t, plan.NeedsRollout)
	assert.Len(t, plan.Allocations, len(ips))
	for _, alloc := range plan.Allocations {
		assert.Equal(t, "restart", alloc.Action, "worker %s", alloc.WorkerIP)
	}

	require.NoError(t, deploy.Execute(nil, deploy.Options{}))
	assert.True(t, jobAllocationHashesPromoted(t, integrationJobName))

	result, err := deploy.DryRun(nil, deploy.Options{})
	require.NoError(t, err)
	assert.False(t, result.Required)

	for _, ip := range ips {
		assert.Equal(t, 1, workerJobCounter(t, ip, "start"), "start count should be unchanged on %s", ip)
		assert.Equal(t, 1, workerJobCounter(t, ip, "restart"), "upgrade deploy should restart once on %s", ip)
	}
}

func TestIntegrationDeployRollingUpgradeParallelism(t *testing.T) {
	ips := workerIPsFromAssets(t)
	if len(ips) < 3 {
		t.Skip("rolling parallelism integration test requires at least three workers in assets/workers.json")
	}

	const parallel = 2
	setupRollingIntegrationBucket(t, parallel)
	assert.Equal(t, parallel, jobUpdateParallelCount(t, integrationJobName))

	require.NoError(t, deploy.Execute(nil, deploy.Options{}))

	marker := filepath.Join(bucket.WorkspaceLocation, "jobs", integrationJobName, "upgrade-marker.txt")
	require.NoError(t, os.WriteFile(marker, []byte("v3"), 0o644))
	executeBuild(t)

	plan := dryRunPlanForJob(t, integrationJobName)
	assert.True(t, plan.NeedsRollout)
	restartWorkers := make(map[string]struct{})
	for _, alloc := range plan.Allocations {
		if alloc.Action == "restart" {
			restartWorkers[alloc.WorkerIP] = struct{}{}
		}
	}
	assert.Len(t, restartWorkers, len(ips))

	require.NoError(t, deploy.Execute(nil, deploy.Options{}))

	for _, ip := range ips {
		assert.Equal(t, 1, workerJobCounter(t, ip, "restart"), "batched upgrade should restart once on %s", ip)
	}
}

// workerIPsFromAssets reads assets/workers.json so tests can skip before bucket init.
func workerIPsFromAssets(t *testing.T) []string {
	t.Helper()
	requireIntegrationAssets(t)
	raw, err := os.ReadFile(filepath.Join(assetsDir(t), "workers.json"))
	require.NoError(t, err)

	var entries []struct {
		Host string `json:"host"`
	}
	require.NoError(t, json.Unmarshal(raw, &entries))

	ips := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Host != "" {
			ips = append(ips, entry.Host)
		}
	}
	return ips
}
