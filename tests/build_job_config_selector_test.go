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

func TestBuildUsesJobConfigSelectorConf(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeMinimalJob(t, "app", `{"selectors":["worker"]}`)

	conf := `[app]
marker = "from-prod-conf"
`
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.jobs.prod.conf"), []byte(conf), 0o644))
	require.NoError(t, os.WriteFile(path.Join(bucket.Location, "maand.conf"), []byte(`ssh_user = "agent"
ssh_key = "worker.key"
use_sudo = true
job_config_selector = "prod"
`), 0o644))

	executeBuild(t)

	value, err := GetKey("vars/bucket/job/app", "marker")
	require.NoError(t, err)
	assert.Equal(t, "from-prod-conf", value)
}
