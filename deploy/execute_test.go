package deploy

import (
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"
	"testing"

	"maand/bucket"
	"maand/data"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute_deploysSingleJob(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, false))

	tx = env.begin(t)
	assert.True(t, env.allocationHashPromoted(t, tx, "app", "alloc-app-10.0.0.1"))
	require.NoError(t, tx.Rollback())
	assert.True(t, rec.HasAction("10.0.0.1", "start", "app"))
}

func TestExecute_deployWithJobFilter(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.seedMakefileJob(t, tx, "other", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute([]string{"app"}, false))

	assert.True(t, rec.HasAction("10.0.0.1", "start", "app"))
	assert.False(t, rec.HasAction("10.0.0.1", "start", "other"))
}

func TestExecute_forceRedeploysPromotedJob(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, false))
	require.NotEmpty(t, rec.Commands)

	rec2 := installNoopDeployHooks(t, env.bucketID)
	require.NoError(t, Execute(nil, true))
	assert.True(t, rec2.HasAction("10.0.0.1", "restart", "app"))
}

func TestExecute_skipsAlreadyPromotedJob(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, false))
	require.NotEmpty(t, rec.Commands)

	rec2 := installNoopDeployHooks(t, env.bucketID)
	require.NoError(t, Execute(nil, false))
	assert.Empty(t, rec2.Commands)
}

func TestExecute_restartsUpdatedAllocations(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "new", "old")
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, false))
	assert.True(t, rec.HasAction("10.0.0.1", "restart", "app"))
	assert.False(t, rec.HasAction("10.0.0.1", "start", "app"))
}

func TestExecute_partialDeployPromotesOnlySuccessfulJob(t *testing.T) {
	env := setupDeployTestEnv(t)
	failJob := "broken"
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, workerIP string, commands []string, _ []string) error {
			for _, cmd := range commands {
				if strings.Contains(cmd, failJob) {
					return &JobError{Job: failJob, Err: errors.New("simulated deploy failure")}
				}
			}
			return nil
		},
		Rsync: func(*bucket.Runtime, string, string) error { return nil },
		SetupRuntime: func(string) (*bucket.Runtime, error) {
			return nil, nil
		},
	})
	t.Cleanup(ClearTestHooks)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "good", "10.0.0.1", 0)
	env.seedMakefileJob(t, tx, failJob, "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	err := Execute(nil, false)
	require.Error(t, err)

	tx = env.begin(t)
	assert.True(t, env.allocationHashPromoted(t, tx, "good", "alloc-good-10.0.0.1"))
	assert.False(t, env.allocationHashPromoted(t, tx, failJob, "alloc-broken-10.0.0.1"))
	require.NoError(t, tx.Rollback())

	// Retry deploy should continue broken only (good stays skipped)
	rec := installNoopDeployHooks(t, env.bucketID)
	require.NoError(t, Execute(nil, false))
	assert.True(t, rec.HasAction("10.0.0.1", "start", failJob))
	assert.False(t, rec.HasAction("10.0.0.1", "start", "good"))
}

func TestExecute_stopsRemovedAllocationAndPreservesDataLogs(t *testing.T) {
	env := setupDeployTestEnv(t)
	var recorded []string
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, workerIP string, commands []string, _ []string) error {
			for _, cmd := range commands {
				recorded = append(recorded, workerIP+":"+cmd)
			}
			return nil
		},
		Rsync: func(*bucket.Runtime, string, string) error { return nil },
		SetupRuntime: func(string) (*bucket.Runtime, error) {
			return nil, nil
		},
	})
	t.Cleanup(ClearTestHooks)

	tx := env.begin(t)
	env.insertWorker(t, tx, "10.0.0.1", 0)
	jobID := env.insertJob(t, tx, "gone", 0, 1)
	env.insertAllocation(t, tx, "alloc-gone", "10.0.0.1", "gone", 0, 0, 1)
	env.setAllocationHash(t, tx, "gone", "alloc-gone", "h1", "h0")
	env.insertJobFile(t, tx, jobID, "gone", "", true)
	env.insertJobFile(t, tx, jobID, path.Join("gone", "Makefile"), makefileContent(), false)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, false))

	joined := strings.Join(recorded, "\n")
	assert.Contains(t, joined, runnerCommand(env.bucketID, "stop", "gone"))
	assert.Contains(t, joined, "/opt/worker/"+env.bucketID+"/jobs/gone")
	assert.Contains(t, joined, "! -name data")
	assert.Contains(t, joined, "! -name logs")
	assert.NotContains(t, joined, "rm -rf /opt/worker/"+env.bucketID+"/jobs/gone")

	tx = env.begin(t)
	var hashCount int
	require.NoError(t, tx.QueryRow(
		`SELECT count(*) FROM hash WHERE namespace = 'gone_allocation' AND key = 'alloc-gone'`,
	).Scan(&hashCount))
	assert.Equal(t, 0, hashCount)
	require.NoError(t, tx.Rollback())
}

func TestExecute_removesBucketFromOffCatalogWorker(t *testing.T) {
	env := setupDeployTestEnv(t)
	var recorded []string
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, workerIP string, commands []string, _ []string) error {
			for _, cmd := range commands {
				recorded = append(recorded, workerIP+":"+cmd)
			}
			return nil
		},
		Rsync: func(*bucket.Runtime, string, string) error { return nil },
		SetupRuntime: func(string) (*bucket.Runtime, error) {
			return nil, nil
		},
	})
	t.Cleanup(ClearTestHooks)

	tx := env.begin(t)
	env.insertWorker(t, tx, "10.0.0.1", 0)
	jobID := env.insertJob(t, tx, "app", 0, 1)
	env.insertAllocation(t, tx, "alloc-gone", "10.0.0.99", "app", 0, 0, 1)
	env.setAllocationHash(t, tx, "app", "alloc-gone", "h1", "h0")
	env.insertJobFile(t, tx, jobID, path.Join("app", "Makefile"), makefileContent(), false)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, false))

	joined := strings.Join(recorded, "\n")
	assert.Contains(t, joined, runnerCommand(env.bucketID, "stop", "app"))
	assert.Contains(t, joined, "10.0.0.99:")
	assert.Contains(t, joined, "/opt/worker/"+env.bucketID+"/jobs/app")
	assert.Contains(t, joined, "! -name data")
	assert.NotContains(t, joined, "rm -rf /opt/worker/"+env.bucketID+"/jobs/app")
	assert.Contains(t, joined, "10.0.0.99:rm -rf /opt/worker/"+env.bucketID)
}

func TestExecute_offCatalogWorkerUnreachableAssumedDead(t *testing.T) {
	env := setupDeployTestEnv(t)
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, workerIP string, _ []string, _ []string) error {
			if workerIP == "10.0.0.99" {
				return fmt.Errorf("ssh: connect to host 10.0.0.99 port 22: connection refused")
			}
			return nil
		},
		Rsync: func(*bucket.Runtime, string, string) error { return nil },
		SetupRuntime: func(string) (*bucket.Runtime, error) {
			return nil, nil
		},
	})
	t.Cleanup(ClearTestHooks)

	tx := env.begin(t)
	env.insertWorker(t, tx, "10.0.0.1", 0)
	jobID := env.insertJob(t, tx, "app", 0, 1)
	env.insertAllocation(t, tx, "alloc-gone", "10.0.0.99", "app", 0, 0, 1)
	env.setAllocationHash(t, tx, "app", "alloc-gone", "h1", "h0")
	env.insertJobFile(t, tx, jobID, path.Join("app", "Makefile"), makefileContent(), false)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, false))
}

func TestExecute_respectsDeploymentSequence(t *testing.T) {
	env := setupDeployTestEnv(t)
	var order []string
	SetTestHooks(&TestHooks{
		WorkerCommand: func(_ *bucket.Runtime, _ string, commands []string, _ []string) error {
			for _, cmd := range commands {
				if strings.Contains(cmd, " --jobs ") {
					order = append(order, cmd[strings.LastIndex(cmd, "--jobs ")+7:])
				}
			}
			return nil
		},
		Rsync: func(*bucket.Runtime, string, string) error { return nil },
		SetupRuntime: func(string) (*bucket.Runtime, error) {
			return nil, nil
		},
	})
	t.Cleanup(ClearTestHooks)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "seq2", "10.0.0.1", 2)
	env.seedMakefileJob(t, tx, "seq0", "10.0.0.1", 0)
	env.seedMakefileJob(t, tx, "seq1", "10.0.0.1", 1)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, false))
	require.GreaterOrEqual(t, len(order), 3)
	first := order[0]
	assert.Equal(t, "seq0", first)
}

func TestExecute_notInitialized(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	err := Execute(nil, false)
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrNotInitialized)
}

func TestUpdateSeq_increments(t *testing.T) {
	env := setupDeployTestEnv(t)
	require.NoError(t, UpdateSeq(env.db))
	require.NoError(t, UpdateSeq(env.db))

	tx := env.begin(t)
	seq, err := data.GetBucketUpdateSeq(tx)
	require.NoError(t, err)
	assert.Equal(t, 2, seq)
	require.NoError(t, tx.Rollback())
}

func MustQueryCountDeploy(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRow(query, args...).Scan(&count))
	return count
}
