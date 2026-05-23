package deploy

import (
	"testing"

	"maand/data"
	"maand/kv"

	"github.com/stretchr/testify/require"
)

func TestDeployJobWithCommands_returnsErrorWithoutRuntime(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	_, err := tx.Exec(
		`INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
		 VALUES ('job-app', 'app', 'ctl', 'job_control', '', '', '{}')`,
	)
	require.NoError(t, err)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "b", "a")
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, prepareJobsFiles(tx, []string{"app"}))
	require.NoError(t, updateAllocationHash(tx, []string{"app"}))

	commands, err := data.GetJobCommands(tx, "app", "job_control")
	require.NoError(t, err)
	err = deployJobWithCommands(tx, nil, "app", commands)
	require.Error(t, err)
	require.NoError(t, tx.Rollback())
}
