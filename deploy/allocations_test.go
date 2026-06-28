package deploy

import (
	"path"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleNewAllocations_startsUnpromotedAllocation(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, prepareJobsFiles(tx, []string{"app"}))
	require.NoError(t, updateAllocationHash(tx, []string{"app"}))
	require.NoError(t, handleNewAllocations(tx, nil, env.bucketID, "app"))
	assert.True(t, rec.HasAction("10.0.0.1", "start", "app"))
	require.NoError(t, tx.Rollback())
}

func TestActiveWorkers_excludesDisabled(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.ensureWorker(t, tx, "10.0.0.1", 0)
	env.ensureWorker(t, tx, "10.0.0.2", 1)
	env.insertJob(t, tx, "app", 0, 1)
	env.insertAllocation(t, tx, "a1", "10.0.0.1", "app", 0, 0, 0)
	env.insertAllocation(t, tx, "a2", "10.0.0.2", "app", 0, 1, 0)

	active, err := activeWorkers([]string{"10.0.0.1", "10.0.0.2"}, tx, "app")
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1"}, active)
	require.NoError(t, tx.Rollback())
}

func TestCountActiveAllocations(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.insertAllocation(t, tx, "a2", "10.0.0.2", "app", 0, 0, 1)

	count, err := countActiveAllocations(tx, "app")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	require.NoError(t, tx.Rollback())
}

func TestGetNewAllocations_afterReenable(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "cur", "cur")
	require.NoError(t, data.MarkAllocationStartPending(tx, "app", "alloc-app-10.0.0.1"))

	workers, err := data.GetNewAllocations(tx, "app")
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1"}, workers)
	require.NoError(t, tx.Rollback())
}

func TestAllocationsNeedingRestart_forceIncludesActive(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	workers, err := allocationsNeedingRestart(tx, "app", true)
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1"}, workers)
	require.NoError(t, tx.Rollback())
}

func TestHandleUpdatedAllocations_restartsChangedAllocation(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "new", "old")
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, handleUpdatedAllocations(tx, nil, env.bucketID, "app", Options{}))
	assert.True(t, rec.HasAction("10.0.0.1", "restart", "app"))
	require.NoError(t, tx.Rollback())
}

func TestAllocationsNeedingRestart_includesVersionPending(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "same", "same")
	_, err := tx.Exec(`UPDATE hash SET current_version = '1.0.0' WHERE namespace = 'app_allocation' AND key = 'alloc-app-10.0.0.1'`)
	require.NoError(t, err)
	_, err = tx.Exec(`UPDATE allocations SET new_version = '2.0.0' WHERE alloc_id = 'alloc-app-10.0.0.1'`)
	require.NoError(t, err)

	workers, err := allocationsNeedingRestart(tx, "app", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1"}, workers)
	require.NoError(t, tx.Rollback())
}

// On a fresh deploy a new allocation (previous_hash NULL) has an empty current_version,
// so it looks version-pending vs the build target. It must NOT be in the restart set:
// handleNewAllocations already started it, and restarting would be a redundant second rollout.
func TestAllocationsNeedingRestart_excludesNewVersionPending(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "h", "")
	_, err := tx.Exec(`UPDATE allocations SET new_version = '1.0.0' WHERE alloc_id = 'alloc-app-10.0.0.1'`)
	require.NoError(t, err)

	newAllocs, err := data.GetNewAllocations(tx, "app")
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.1"}, newAllocs)

	versionPending, err := data.GetVersionPendingAllocations(tx, "app")
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.1"}, versionPending)

	workers, err := allocationsNeedingRestart(tx, "app", false)
	require.NoError(t, err)
	assert.Empty(t, workers)
	require.NoError(t, tx.Rollback())
}

func TestHandleStoppedAllocations_disabledStopsWithoutRemovingArtifacts(t *testing.T) {
	env := setupDeployTestEnv(t)
	var stopped, removedArtifacts bool
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, _ string, _ bucket.CommandContext, commands []string, _ []string) error {
			for _, c := range commands {
				if containsCmd(c, "stop") {
					stopped = true
				}
				if containsCmd(c, "find") && containsCmd(c, "-exec rm") {
					removedArtifacts = true
				}
			}
			return nil
		},
		Rsync:        func(*bucket.Runtime, string, string, []string) error { return nil },
		SetupRuntime: func(string, bucket.RunContext) (*bucket.Runtime, error) { return nil, nil },
	})
	t.Cleanup(ClearTestHooks)

	tx := env.begin(t)
	env.ensureWorker(t, tx, "10.0.0.1", 0)
	jobID := env.insertJob(t, tx, "paused", 0, 1)
	env.insertAllocation(t, tx, "alloc-paused", "10.0.0.1", "paused", 0, 1, 0)
	env.setAllocationHash(t, tx, "paused", "alloc-paused", "c", "p")
	env.insertJobFile(t, tx, jobID, path.Join("paused", "Makefile"), makefileContent(), false)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, handleStoppedAllocations(tx, nil, env.bucketID, []string{"paused"}))
	assert.True(t, stopped)
	assert.False(t, removedArtifacts)
	require.NoError(t, tx.Rollback())
}

func TestHandleStoppedAllocations_stopsRemoved(t *testing.T) {
	env := setupDeployTestEnv(t)
	var stopped bool
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, _ string, _ bucket.CommandContext, commands []string, _ []string) error {
			for _, c := range commands {
				if containsCmd(c, "stop") {
					stopped = true
				}
			}
			return nil
		},
		Rsync: func(*bucket.Runtime, string, string, []string) error { return nil },
		SetupRuntime: func(string, bucket.RunContext) (*bucket.Runtime, error) { return nil, nil },
	})
	t.Cleanup(ClearTestHooks)

	tx := env.begin(t)
	env.ensureWorker(t, tx, "10.0.0.1", 0)
	jobID := env.insertJob(t, tx, "gone", 0, 1)
	env.insertAllocation(t, tx, "alloc-gone", "10.0.0.1", "gone", 0, 0, 1)
	env.setAllocationHash(t, tx, "gone", "alloc-gone", "c", "p")
	env.insertJobFile(t, tx, jobID, path.Join("gone", "Makefile"), makefileContent(), false)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, handleStoppedAllocations(tx, nil, env.bucketID, []string{"gone"}))
	assert.True(t, stopped)
	require.NoError(t, tx.Rollback())
}

func containsCmd(cmd, sub string) bool {
	return len(cmd) >= len(sub) && (sub == "" || indexSubstring(cmd, sub) >= 0)
}

func indexSubstring(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
