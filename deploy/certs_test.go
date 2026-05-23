package deploy

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateCerts_writesWorkerCerts(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	jobID := "job-app"
	_, err := tx.Exec(`INSERT INTO job_certs (job_id, name) VALUES (?, 'tls')`, jobID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	ns := "maand/job/app/worker/10.0.0.1"
	store.Put(ns, "certs/tls.crt", "CRT", 0)
	store.Put(ns, "certs/tls.key", "KEY", 0)
	require.NoError(t, prepareJobsFiles(tx, []string{"app"}))
	require.NoError(t, updateCerts(tx, "app", "10.0.0.1"))
	require.NoError(t, tx.Rollback())

	certsDir := path.Join(bucket.GetTempWorkerPath("10.0.0.1"), "jobs", "app", "certs")
	crt, err := os.ReadFile(path.Join(certsDir, "tls.crt"))
	require.NoError(t, err)
	assert.Equal(t, "CRT", string(crt))
	key, err := os.ReadFile(path.Join(certsDir, "tls.key"))
	require.NoError(t, err)
	assert.Equal(t, "KEY", string(key))
	assert.FileExists(t, path.Join(certsDir, "ca.crt"))
}
