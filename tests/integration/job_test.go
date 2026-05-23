// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"testing"

	"maand/data"
	"maand/deploy"
	"maand/healthcheck"
	"maand/jobcommand"
	"maand/jobcontrol"
	"maand/runcommand"

	"github.com/stretchr/testify/require"
)

func TestIntegrationJobControlLifecycle(t *testing.T) {
	setupIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil))

	require.NoError(t, jobcontrol.Execute(integrationJobName, "", "stop", false))
	require.NoError(t, jobcontrol.Execute(integrationJobName, "", "start", false))
	require.NoError(t, jobcontrol.Execute(integrationJobName, "", "restart", false))
	require.NoError(t, jobcontrol.Execute(integrationJobName, "", "status", false))
}

func TestIntegrationJobRunCustomTarget(t *testing.T) {
	setupIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil))

	require.NoError(t, jobcontrol.Execute(integrationJobName, "", "migrate", false))

	bucketID := mustBucketID(t)
	worker := workerIPs(t)[0]
	cmd := "cat /opt/worker/" + bucketID + "/jobs/" + integrationJobName + "/data/migrate"
	require.NoError(t, runcommand.Execute(worker, "", 1, cmd, false))
}

func TestIntegrationJobControlSingleWorker(t *testing.T) {
	setupIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil))

	worker := workerIPs(t)[0]
	require.NoError(t, jobcontrol.Execute(integrationJobName, worker, "stop", false))
	require.NoError(t, jobcontrol.Execute(integrationJobName, worker, "start", false))
}

func TestIntegrationHealthCheck(t *testing.T) {
	setupFullIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil))

	require.NoError(t, healthcheck.Execute(false, true, integrationJobName))
}

func TestIntegrationJobCommandCLI(t *testing.T) {
	setupFullIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil))

	require.NoError(t, jobcommand.Execute(integrationJobName, "command_cli", "cli", 2, true, nil))
}

func TestIntegrationJobCommandCLIKV(t *testing.T) {
	setupFullIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil))

	require.NoError(t, jobcommand.Execute(integrationJobName, "command_cli_kv", "cli", 2, true, nil))
}

func TestIntegrationJobCommandCLISecret(t *testing.T) {
	setupFullIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil))

	require.NoError(t, jobcommand.Execute(integrationJobName, "command_cli_secret", "cli", 2, true, nil))
}

func TestIntegrationDeployHooks(t *testing.T) {
	setupFullIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil))
}

func TestIntegrationJobControlWithHealthCheck(t *testing.T) {
	setupFullIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil))

	require.NoError(t, jobcontrol.Execute(integrationJobName, "", "restart", true))
}

func TestIntegrationBuildOnly(t *testing.T) {
	setupIntegrationBucket(t)

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
}
