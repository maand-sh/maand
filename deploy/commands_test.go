package deploy

import (
	"testing"

	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutePreJobCommandsForJob_noCommands(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, executePreJobCommandsForJob(tx, nil, "app"))
	require.NoError(t, tx.Rollback())
}

func TestExecutePostJobCommands_noCommands(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, executePostJobCommands(tx, nil, "app"))
	require.NoError(t, tx.Rollback())
}

func TestExecutePostJobCommands_commandFailure(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	_, err := tx.Exec(
		`INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
		 VALUES ('job-app', 'app', 'broken_post', 'post_deploy', '', '', '{}')`,
	)
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	err = executePostJobCommands(tx, nil, "app")
	require.Error(t, err)
	var jobErr *JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, "app", jobErr.Job)
	require.NoError(t, tx.Rollback())
}

func TestExecutePreJobCommands_stopsOnFirstError(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "good", "10.0.0.1", 0)
	_, err := tx.Exec(
		`INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
		 VALUES ('job-good', 'good', 'broken_pre', 'pre_deploy', '', '', '{}')`,
	)
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	err = executePreJobCommands(tx, nil, []string{"good", "missing"})
	require.Error(t, err)
	var jobErr *JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, "good", jobErr.Job)
	require.NoError(t, tx.Rollback())
}

func TestExecutePreJobCommands_multipleJobs(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "a", "10.0.0.1", 0)
	env.seedMakefileJob(t, tx, "b", "10.0.0.1", 0)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, executePreJobCommands(tx, nil, []string{"a", "b"}))
	require.NoError(t, tx.Rollback())
}
