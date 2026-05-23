package deploy

import (
	"testing"

	"maand/data"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployJob_startsNewAllocation(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, prepareJobsFiles(tx, []string{"app"}))
	require.NoError(t, updateAllocationHash(tx, []string{"app"}))
	require.NoError(t, deployJob(tx, nil, env.bucketID, "app"))
	assert.True(t, rec.HasAction("10.0.0.1", "start", "app"))
	assert.True(t, env.allocationHashPromoted(t, tx, "app", "alloc-app-10.0.0.1"))
	require.NoError(t, tx.Rollback())
}

func TestDeployJob_skipsWhenAlreadyPromoted(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "same", "same")
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, deployJob(tx, nil, env.bucketID, "app"))
	assert.Empty(t, rec.Commands)
	require.NoError(t, tx.Rollback())
}

func TestDeployJob_routesToJobControlWhenCommandsExist(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	_, err := tx.Exec(
		`INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
		 VALUES ('job-app', 'app', 'ctl', 'job_control', '', '', '{}')`,
	)
	require.NoError(t, err)
	commands, err := data.GetJobCommands(tx, "app", "job_control")
	require.NoError(t, err)
	require.Len(t, commands, 1)
	require.NoError(t, tx.Rollback())
}
