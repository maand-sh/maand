// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"fmt"
	"testing"

	"maand/cat"
	"maand/data"

	"github.com/stretchr/testify/require"
)

func TestIntegrationInfoAndCat(t *testing.T) {
	setupFullIntegrationBucket(t)

	require.NoError(t, cat.Info())
	require.NoError(t, cat.Workers())
	require.NoError(t, cat.Jobs())
	require.NoError(t, cat.Allocations("", ""))
	require.NoError(t, cat.JobCommands())
	require.NoError(t, cat.JobPorts())
	require.NoError(t, cat.KV())

	worker := workerIPs(t)[0]
	namespace := fmt.Sprintf("maand/worker/%s", worker)
	require.NoError(t, cat.KVGet(namespace, "worker_ip"))

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
	require.GreaterOrEqual(t, active, 1)
}
