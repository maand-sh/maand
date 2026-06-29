// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/healthcheck"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheckFailsForUnknownJobFilter(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeMinimalJob(t, "app", `{"selectors":["worker"]}`)
	runBuild(t)

	err := healthcheck.Execute(false, false, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jobs not in this bucket")
}

func TestHealthCheckRegistersCommandDuringBuild(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "hcjob")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_modules"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_modules", "command_health_check.py"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["worker"],
		"commands": {
			"command_health_check": {"executed_on": ["health_check"]}
		}
	}`), 0o644))

	runBuild(t)

	count := MustQueryCount(t,
		`SELECT count(*) FROM job_commands WHERE job = 'hcjob' AND executed_on = 'health_check'`,
	)
	assert.Equal(t, 1, count)
}
