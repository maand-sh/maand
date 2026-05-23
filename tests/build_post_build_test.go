// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/build"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFailsWhenPostBuildHooksFail(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "hookjob")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_modules"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_modules", "command_post.py"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["worker"],
		"commands": {
			"command_post": {"executed_on": ["post_build"]}
		}
	}`), 0o644))

	err := build.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "post_build")

	count := MustQueryCount(t,
		`SELECT count(*) FROM job_commands WHERE job = 'hookjob' AND executed_on = 'post_build'`,
	)
	assert.Equal(t, 1, count)
}
