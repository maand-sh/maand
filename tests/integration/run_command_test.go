// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"testing"

	"maand/deploy"
	"maand/runcommand"

	"github.com/stretchr/testify/require"
)

func TestIntegrationRunCommandEcho(t *testing.T) {
	setupIntegrationBucket(t)

	err := runcommand.Execute("", "", 1, "echo maand-integration-ok", false)
	require.NoError(t, err)
}

func TestIntegrationRunCommandAfterDeploy(t *testing.T) {
	setupIntegrationBucket(t)

	require.NoError(t, deploy.Execute(nil, deploy.Options{}))

	err := runcommand.Execute("", "", 1, "echo post-deploy-ok", false)
	require.NoError(t, err)
}

func TestIntegrationRunCommandWorkerFilter(t *testing.T) {
	setupIntegrationBucket(t)

	worker := workerIPs(t)[0]
	err := runcommand.Execute(worker, "", 1, "echo worker-filter-ok", false)
	require.NoError(t, err)
}

func TestIntegrationRunCommandLabelFilter(t *testing.T) {
	setupIntegrationBucket(t)

	err := runcommand.Execute("", "worker", 1, "echo label-filter-ok", false)
	require.NoError(t, err)
}

func TestIntegrationRunCommandWithHealthCheck(t *testing.T) {
	setupFullIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil, deploy.Options{}))

	err := runcommand.Execute("", "", 1, "echo hc-batch-ok", true)
	require.NoError(t, err)
}
