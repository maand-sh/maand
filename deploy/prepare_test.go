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
