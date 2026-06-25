package deploy

import (
	"os"
	"path"
	"testing"

	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranspile_postgresStyleMemoryTemplate(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "postgres", "10.0.0.1", 0)
	tpl := `{{ define "pgMemUnit" -}}
{{- $mb := int . -}}
{{- if ge $mb 1024 -}}
{{- div $mb 1024 -}}GB
{{- else -}}
{{- $mb -}}MB
{{- end -}}
{{- end }}
{{- $clonefrom := eq (get (printf "maand/worker/%s" .WorkerIP) "postgres_allocation_index") "1" -}}
{{- $memMB := int (get "vars/job/postgres" "memory") -}}
{{- $sharedMB := div $memMB 4 -}}
{{- $cacheMB := div (mul $memMB 3) 4 -}}
{{- $maintMB := min 2048 (div $memMB 16) -}}
{{- $workMB := min 128 (max 4 (div $memMB 64)) -}}
clone_from = {{ $clonefrom }}
shared_buffers = {{ template "pgMemUnit" $sharedMB }}
effective_cache_size = {{ template "pgMemUnit" $cacheMB }}
maintenance_work_mem = {{ $maintMB }}MB
work_mem = {{ $workMB }}MB
`
	env.insertJobFile(t, tx, "job-postgres", path.Join("postgres", "postgresql.conf.tpl"), tpl, false)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	store.Put("vars/job/postgres", "memory", "8192", 0)
	store.Put("maand/worker/10.0.0.1", "postgres_allocation_index", "1", 0)
	require.NoError(t, prepareJobsFiles(tx, []string{"postgres"}))
	require.NoError(t, transpile(tx, "postgres", "10.0.0.1"))
	require.NoError(t, tx.Rollback())

	out := path.Join(env.root, "tmp", "workers", "10.0.0.1", "jobs", "postgres", "postgresql.conf")
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Contains(t, string(content), "clone_from = true")
	assert.Contains(t, string(content), "shared_buffers = 2GB")
	assert.Contains(t, string(content), "effective_cache_size = 6GB")
	assert.Contains(t, string(content), "maintenance_work_mem = 512MB")
	assert.Contains(t, string(content), "work_mem = 128MB")
}

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
