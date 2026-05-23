package deploy

import (
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestHandleStoppedAllocations_stopsRemoved(t *testing.T) {
	env := setupDeployTestEnv(t)
	var stopped bool
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, _ string, commands []string, _ []string) error {
			for _, c := range commands {
				if containsCmd(c, "stop") {
					stopped = true
				}
			}
			return nil
		},
		Rsync: func(*bucket.Runtime, string, string) error { return nil },
		SetupRuntime: func(string) (*bucket.Runtime, error) { return nil, nil },
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
