// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"testing"

	"maand/deploy"
	"maand/gc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationGCAfterWorkerRemoval(t *testing.T) {
	setupIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil, false))

	ips := workerIPs(t)
	require.GreaterOrEqual(t, len(ips), 2, "need at least two workers for removal GC test")

	remaining := ips[0]
	replaceWorkersJSON(t, `[{"host":"`+remaining+`"}]`)
	executeBuild(t)

	assert.Equal(t, len(ips), countAllocations(t, false)+countAllocations(t, true))
	assert.GreaterOrEqual(t, countAllocations(t, true), 1)

	require.NoError(t, deploy.Execute(nil, false))
	require.NoError(t, gc.Execute(0))
	assert.Equal(t, 0, countAllocations(t, true))
	assert.Equal(t, 1, countAllocations(t, false))
}

func TestIntegrationGCCollect(t *testing.T) {
	setupIntegrationBucket(t)
	require.NoError(t, deploy.Execute(nil, false))
	require.NoError(t, gc.Collect())
}
