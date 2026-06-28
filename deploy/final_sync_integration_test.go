package deploy

import (
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinalSyncDeployedJobs_writesFilteredRules(t *testing.T) {
	env := setupDeployTestEnv(t)
	var captured string
	SetTestHooks(&TestHooks{
		WorkerCommand: (&CommandRecorder{BucketID: env.bucketID}).Record,
		Rsync: func(_ *bucket.Runtime, _, workerIP string, _ []string) error {
			filterPath := path.Join(bucket.TempLocation, "workers", workerIP+".rsync")
			data, err := os.ReadFile(filterPath)
			if err != nil {
				return err
			}
			captured = string(data)
			return nil
		},
		SetupRuntime: func(string, bucket.RunContext) (*bucket.Runtime, error) { return nil, nil },
	})
	t.Cleanup(ClearTestHooks)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.seedMakefileJob(t, tx, "other", "10.0.0.1", 0)
	require.NoError(t, prepareJobsFiles(tx, []string{"app"}))
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, finalSyncDeployedJobs(tx, nil, env.bucketID, []string{"app"}))
	require.NoError(t, tx.Rollback())

	assert.Contains(t, captured, "+ jobs/app/")
	assert.NotContains(t, captured, "+ jobs/other/")
}
