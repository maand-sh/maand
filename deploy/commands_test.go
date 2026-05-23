package deploy

import (
	"testing"

	"maand/kv"

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

func TestExecutePreJobCommands_multipleJobs(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "a", "10.0.0.1", 0)
	env.seedMakefileJob(t, tx, "b", "10.0.0.1", 0)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, executePreJobCommands(tx, nil, []string{"a", "b"}))
	require.NoError(t, tx.Rollback())
}
