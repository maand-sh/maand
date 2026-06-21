package promconfig

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/initialize"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateScrapeFiles_rejectsBoth(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api", "_prometheus")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, ScrapeFileName), []byte("- job_name: api\n"), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, ScrapeFileTplName), []byte("- job_name: api\n"), 0o644))

	err := ValidateScrapeFiles("api")
	assert.Error(t, err)
}

func TestRenderScrapeTemplate(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, initialize.Execute())

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte("\n"), 0o644))

	db, err := data.OpenDatabase(false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	tx, err := db.Begin()
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	_, err = tx.Exec(`INSERT INTO job (job_id, name, version) VALUES ('job-api', 'api', '1.0.0')`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	store.Put("maand/job/api", "job_name", "api", 0)
	store.Put("vars/job/api", "metrics_path", "/metrics", 0)

	rendered, err := RenderScrapeTemplate(tx, "api", []byte(`
- job_name: {{ .Job }}
  metrics_path: {{ get "vars/job/api" "metrics_path" }}
  static_configs:
    - targets:
        - 127.0.0.1:9090
      labels:
        maand_job: {{ get "maand/job/api" "job_name" }}
`))
	require.NoError(t, err)

	configs, err := ParseScrapeFile(rendered)
	require.NoError(t, err)
	assert.Equal(t, "api", configs[0]["job_name"])
	assert.Equal(t, "/metrics", configs[0]["metrics_path"])
}
