package deploy

import (
	"path"
	"strings"
	"testing"

	"maand/bucket"
	"maand/data"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute_disabledPromotedAllocationStopsOnDeploy(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, Options{}))
	assert.True(t, rec.HasAction("10.0.0.1", "start", "app"))

	rec2 := installNoopDeployHooks(t, env.bucketID)
	tx = env.begin(t)
	_, err := tx.Exec(`UPDATE allocations SET disabled = 1 WHERE job = 'app' AND worker_ip = '10.0.0.1'`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, Options{}))
	assert.True(t, rec2.HasAction("10.0.0.1", "stop", "app"))
}

func TestExecute_disabledAllocationDoesNotStart(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	_, err := tx.Exec(`UPDATE allocations SET disabled = 1 WHERE job = 'app' AND worker_ip = '10.0.0.1'`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, Options{}))
	assert.Empty(t, rec.Commands)
}

func TestExecute_addsSecondAllocationOnLaterDeploy(t *testing.T) {
	env := setupDeployTestEnv(t)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	rec := installNoopDeployHooks(t, env.bucketID)
	require.NoError(t, Execute(nil, Options{}))
	assert.True(t, rec.HasAction("10.0.0.1", "start", "app"))

	tx = env.begin(t)
	env.ensureWorker(t, tx, "10.0.0.2", 1)
	env.insertAllocation(t, tx, "alloc-app-2", "10.0.0.2", "app", 0, 0, 0)
	env.insertJobFile(t, tx, "job-app", path.Join("app", "Makefile"), makefileContent(), false)
	require.NoError(t, tx.Commit())

	rec = installNoopDeployHooks(t, env.bucketID)
	require.NoError(t, Execute(nil, Options{}))
	assert.True(t, rec.HasAction("10.0.0.2", "start", "app"))
}

func TestExecute_threeJobsOrderedByDeploymentSequence(t *testing.T) {
	env := setupDeployTestEnv(t)
	var order []string
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, workerIP string, _ bucket.CommandContext, commands []string, _ []string) error {
			_ = workerIP
			for _, cmd := range commands {
				if idx := indexJobFlag(cmd); idx >= 0 {
					order = append(order, cmd[idx:])
				}
			}
			return nil
		},
		Rsync:          func(*bucket.Runtime, string, string, []string) error { return nil },
		SetupRuntime: func(string, bucket.RunContext) (*bucket.Runtime, error) { return nil, nil },
	})
	t.Cleanup(ClearTestHooks)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "seq2", "10.0.0.1", 2)
	env.seedMakefileJob(t, tx, "seq0", "10.0.0.1", 0)
	env.seedMakefileJob(t, tx, "seq1", "10.0.0.1", 1)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, Options{}))
	require.GreaterOrEqual(t, len(order), 3)
	assert.Equal(t, "seq0", order[0])
}

func TestExecute_removedJobWithoutGC(t *testing.T) {
	env := setupDeployTestEnv(t)
	installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "removed", "10.0.0.1", 0)
	_, err := tx.Exec(`DELETE FROM job WHERE name = 'removed'`)
	require.NoError(t, err)
	_, err = tx.Exec(`UPDATE allocations SET removed = 1 WHERE job = 'removed'`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, Options{}))
}

func indexJobFlag(cmd string) int {
	const flag = "--jobs "
	if i := indexSubstring(cmd, flag); i >= 0 {
		return i + len(flag)
	}
	return -1
}

func TestExecute_rollingUpgradeParallelismFromJob(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.ensureWorker(t, tx, "10.0.0.2", 1)
	env.ensureWorker(t, tx, "10.0.0.3", 2)
	env.insertAllocation(t, tx, "alloc-10.0.0.2", "10.0.0.2", "app", 0, 0, 0)
	env.insertAllocation(t, tx, "alloc-10.0.0.3", "10.0.0.3", "app", 0, 0, 0)
	_, err := tx.Exec(`UPDATE job SET max_concurrent_upgrades = 2 WHERE name = 'app'`)
	require.NoError(t, err)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "n", "o")
	env.setAllocationHash(t, tx, "app", "alloc-10.0.0.2", "n", "o")
	env.setAllocationHash(t, tx, "app", "alloc-10.0.0.3", "n", "o")
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, Options{}))
	restarts := 0
	for _, c := range rec.Commands {
		if strings.Contains(c.Command, "restart") && strings.Contains(c.Command, "app") {
			restarts++
		}
	}
	assert.Equal(t, 3, restarts)

	tx = env.begin(t)
	parallelism, err := data.GetMaxConcurrentUpgrades(tx, "app")
	require.NoError(t, err)
	assert.Equal(t, 2, parallelism)
	require.NoError(t, tx.Rollback())
}
