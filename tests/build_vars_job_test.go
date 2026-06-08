package tests

import (
	"database/sql"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/data"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustGetPersistedKV(t *testing.T, namespace, key string) string {
	t.Helper()
	var value string
	var deleted int
	err := withDatabase(func(db *sql.DB) error {
		return db.QueryRow(`
			SELECT value, deleted FROM key_value
			WHERE namespace = ? AND key = ?
			ORDER BY version DESC LIMIT 1`, namespace, key).Scan(&value, &deleted)
	})
	require.NoError(t, err)
	require.Equal(t, 0, deleted)
	return value
}

func seedVarsJobKey(t *testing.T, jobName, key, value string) {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() {
		_ = tx.Rollback()
	}()

	require.NoError(t, kv.Initialize(tx))
	kv.GetKVStore().Put("vars/job/"+jobName, key, value, 0)
	require.NoError(t, kv.PersistSession())
}

func TestVarsJobFromWorkspaceSurvivesRebuild(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1","labels":["app"]}]`)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "app")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["app"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "vars.toml"), []byte(`cluster_name = "prod"
dc = "us-east"
`), 0o644))

	require.NoError(t, build.Execute())
	assert.Equal(t, "prod", mustGetPersistedKV(t, "vars/job/app", "cluster_name"))
	assert.Equal(t, "us-east", mustGetPersistedKV(t, "vars/job/app", "dc"))

	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["app"],"version":"2"}`), 0o644))
	require.NoError(t, build.Execute())

	assert.Equal(t, "prod", mustGetPersistedKV(t, "vars/job/app", "cluster_name"))
}

func TestVarsJobKVPutSurvivesRebuild(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1","labels":["app"]}]`)
	writeMinimalJob(t, "app", `{"selectors":["app"]}`)

	require.NoError(t, build.Execute())
	seedVarsJobKey(t, "app", "from_script", "stable")

	require.NoError(t, build.Execute())
	assert.Equal(t, "stable", mustGetPersistedKV(t, "vars/job/app", "from_script"))
}

func TestVarsJobWorkspaceMergeDoesNotDeleteScriptKeys(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1","labels":["app"]}]`)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "app")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["app"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "vars.toml"), []byte(`tracked = "yes"`), 0o644))

	seedVarsJobKey(t, "app", "runtime_only", "keep-me")

	require.NoError(t, build.Execute())
	assert.Equal(t, "yes", mustGetPersistedKV(t, "vars/job/app", "tracked"))
	assert.Equal(t, "keep-me", mustGetPersistedKV(t, "vars/job/app", "runtime_only"))
}

func TestBuildPurgesJobCommandKVWhenFlagSetAndNoActiveAllocations(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1","labels":["vault"]}]`)
	writeMinimalJob(t, "vault", `{"selectors":["vault"],"version":"1.0.0"}`)
	require.NoError(t, build.Execute())

	seedVarsJobKey(t, "vault", "cluster_initialized", "true")
	seedJobKV(t, "vault", "secrets/job/vault", "root_token", "enc:v1:token")

	require.NoError(t, os.WriteFile(
		path.Join(bucket.WorkspaceLocation, "jobs", "vault", "manifest.json"),
		[]byte(`{"selectors":["nomatch"],"version":"1.0.0"}`),
		0o644,
	))
	require.NoError(t, build.Execute(build.Options{PurgeJobCommandKV: true}))

	assertKVNamespaceDeleted(t, "vars/job/vault", "cluster_initialized")
	assertKVNamespaceDeleted(t, "secrets/job/vault", "root_token")
}

func TestBuildDoesNotPurgeJobCommandKVWhenNoActiveAllocations(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1","labels":["vault"]}]`)
	writeMinimalJob(t, "vault", `{"selectors":["vault"],"version":"1.0.0"}`)
	require.NoError(t, build.Execute())

	seedVarsJobKey(t, "vault", "cluster_initialized", "true")
	seedJobKV(t, "vault", "secrets/job/vault", "root_token", "enc:v1:token")

	require.NoError(t, os.WriteFile(
		path.Join(bucket.WorkspaceLocation, "jobs", "vault", "manifest.json"),
		[]byte(`{"selectors":["nomatch"],"version":"1.0.0"}`),
		0o644,
	))
	require.NoError(t, build.Execute())

	assert.Equal(t, "true", mustGetPersistedKV(t, "vars/job/vault", "cluster_initialized"))
	assert.Equal(t, "enc:v1:token", mustGetPersistedKV(t, "secrets/job/vault", "root_token"))
}
