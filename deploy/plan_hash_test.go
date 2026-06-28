// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"path"
	"testing"

	"maand/kv"

	"github.com/stretchr/testify/require"
)

func TestRefreshPlanHashesForJobs_noJobs(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	require.NoError(t, refreshPlanHashesForJobs(tx, nil))
	require.NoError(t, tx.Rollback())
}

func TestRefreshPlanHashesForJobPlan_runsPreDeploy(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	_, err := tx.Exec(
		`INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
		 VALUES ('job-app', 'app', 'broken_pre', 'pre_deploy', '', '', '{}')`,
	)
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))

	err = refreshPlanHashesForJobPlan(tx, nil, "app")
	require.Error(t, err)
	var jobErr *JobError
	require.ErrorAs(t, err, &jobErr)
	require.Equal(t, "app", jobErr.Job)
	require.NoError(t, tx.Rollback())
}

func TestRefreshPlanHashesForJobs_detectsContentChange(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, refreshPlanHashesForJobs(tx, []string{"app"}))
	require.NoError(t, promoteAllocationHash(tx, "app"))
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	env.insertJobFile(t, tx, "job-app", path.Join("app", "marker.txt"), "v2", false)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, refreshPlanHashesForJobs(tx, []string{"app"}))
	needs, err := JobNeedsRollout(tx, "app")
	require.NoError(t, err)
	require.True(t, needs, "deploy refresh should detect content change vs promoted hash")
	require.NoError(t, tx.Rollback())
}
