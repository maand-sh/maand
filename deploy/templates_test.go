package deploy

import (
	"os"
	"path"
	"testing"

	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranspile_rendersTemplate(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	jobID := "job-app"
	tpl := `hello {{ get "vars/job/app" "name" }}`
	env.insertJobFile(t, tx, jobID, path.Join("app", "config.tpl"), tpl, false)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	store.Put("vars/job/app", "name", "world", 0)
	require.NoError(t, prepareJobsFiles(tx, []string{"app"}))
	require.NoError(t, transpile(tx, "app", "10.0.0.1"))
	require.NoError(t, tx.Rollback())

	out := path.Join(env.root, "tmp", "workers", "10.0.0.1", "jobs", "app", "config")
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(content))
}
