package deploy

import (
	"errors"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/require"
)

func TestCheckWorkerPrerequisitesFailsBeforeDeploy(t *testing.T) {
	env := setupDeployTestEnv(t)
	SetTestHooks(&TestHooks{
		CheckWorkerPrerequisites: func(*bucket.Runtime, []string) error {
			return errors.New("worker 10.0.0.2 missing prerequisites: make")
		},
		WorkerCommand: func(*bucket.Runtime, string, []string, []string) error { return nil },
		Rsync:         func(*bucket.Runtime, string, string) error { return nil },
		SetupRuntime:  func(string) (*bucket.Runtime, error) { return nil, nil },
	})
	t.Cleanup(ClearTestHooks)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	err := Execute(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing prerequisites: make")
}
