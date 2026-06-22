// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFailsWhenJobHasReservedDataDirectory(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeMinimalJob(t, "app", `{"selectors":["worker"]}`)
	require.NoError(t, os.Mkdir(path.Join(bucket.WorkspaceLocation, "jobs", "app", "data"), 0o755))

	err := executeBuildErr(t)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "data directory is reserved")
}

func TestBuildFailsWhenJobHasReservedLogsDirectory(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeMinimalJob(t, "app", `{"selectors":["worker"]}`)
	require.NoError(t, os.Mkdir(path.Join(bucket.WorkspaceLocation, "jobs", "app", "logs"), 0o755))

	err := executeBuildErr(t)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logs directory is reserved")
}

func TestBuildFailsWhenJobHasReservedBinDirectory(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeMinimalJob(t, "app", `{"selectors":["worker"]}`)
	require.NoError(t, os.Mkdir(path.Join(bucket.WorkspaceLocation, "jobs", "app", "bin"), 0o755))

	err := executeBuildErr(t)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bin directory is reserved")
}
