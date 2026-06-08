package deploy

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareOneWorkerFiles_writesJobsJSON(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.insertAllocation(t, tx, "a2", "10.0.0.1", "removed", 0, 0, 1)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, prepareOneWorkerFiles(tx, "10.0.0.1"))
	require.NoError(t, tx.Rollback())

	data, err := os.ReadFile(path.Join(env.root, "tmp", "workers", "10.0.0.1", "jobs.json"))
	require.NoError(t, err)
	var entries []WorkerJobs
	require.NoError(t, json.Unmarshal(data, &entries))
	if len(entries) != 1 || entries[0].Job != "app" {
		t.Fatalf("jobs.json %#v", entries)
	}
}

func TestGetWorkerData(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	data, err := getWorkerData(tx, "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, env.bucketID, data.BucketID)
	assert.Equal(t, "10.0.0.1", data.WorkerIP)
	assert.NotEmpty(t, data.WorkerID)
	require.NoError(t, tx.Rollback())
}

func TestPrepareJobOnWorker_writesModuleHelpers(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	jobID := env.insertJob(t, tx, "app", 0, 1)
	env.ensureWorker(t, tx, "10.0.0.1", 0)
	env.insertAllocation(t, tx, "a1", "10.0.0.1", "app", 0, 0, 0)
	env.insertJobFile(t, tx, jobID, path.Join("app", "Makefile"), makefileContent(), false)
	env.insertJobFile(t, tx, jobID, path.Join("app", "_modules"), "", true)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, prepareJobOnWorker(tx, "app", "10.0.0.1"))
	require.NoError(t, tx.Rollback())

	moduleDir := path.Join(env.root, "tmp", "workers", "10.0.0.1", "jobs", "app", "_modules")
	assert.FileExists(t, path.Join(moduleDir, "maand.py"))
	assert.FileExists(t, path.Join(moduleDir, "maand.ts"))
}

func TestPrepareWorkersFiles_stagesAllWorkers(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.ensureWorker(t, tx, "10.0.0.2", 1)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, prepareWorkersFiles(tx, []string{"10.0.0.1", "10.0.0.2"}))
	require.NoError(t, tx.Rollback())

	for _, workerIP := range []string{"10.0.0.1", "10.0.0.2"} {
		assert.FileExists(t, path.Join(env.root, "tmp", "workers", workerIP, "jobs.json"))
	}
}

func TestPrepareJobsFiles_stagesMakefile(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, prepareJobsFiles(tx, []string{"app"}))
	require.NoError(t, tx.Rollback())

	makefile := path.Join(env.root, "tmp", "workers", "10.0.0.1", "jobs", "app", "Makefile")
	assert.FileExists(t, makefile)
}
