// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/initialize"
	"maand/kv"
	"maand/promconfig"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedPrometheusCatalog(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`
		INSERT INTO job (job_id, name) VALUES ('job-api', 'api'), ('job-web', 'web');
		INSERT INTO job_files (job_id, path, content, isdir) VALUES
			('job-api', 'api/manifest.json', '{"resources":{"ports":{}}}', 0),
			('job-api', 'api/_prometheus/scrape.yaml', '- job_name: api', 0),
			('job-api', 'api/_prometheus/alerts/slo.yaml', 'groups: []', 0),
			('job-api', 'api/_prometheus/runbooks/ApiDown.md', '# down', 0),
			('job-web', 'web/_prometheus/alerts/errors.yaml', 'groups: []', 0);
	`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
}

func TestPrometheusListAndGet(t *testing.T) {
	seedPrometheusCatalog(t)

	require.NoError(t, Prometheus(""))
	require.NoError(t, Prometheus("api"))
	assert.Error(t, Prometheus("missing"))

	require.NoError(t, PrometheusGet("api", "scrape.yaml"))
	assert.Error(t, PrometheusGet("api", "missing.yaml"))
}

func TestPrometheusGetFromWorkspace(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, initialize.Execute())

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "keeper")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml"), []byte("from-workspace\n"), 0o644))

	require.NoError(t, PrometheusGet("keeper", "scrape"))
}

func TestPrometheusScrapeFromWorkspace(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, initialize.Execute())

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "keeper")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["worker"],
		"resources": {"ports": {"metrics_port": {}}}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml"), []byte(`
- job_name: keeper
  static_configs:
    - targets:
        - 127.0.0.1:9100
`), 0o644))

	require.NoError(t, PrometheusScrape("keeper"))
}

func TestPrometheusScrape(t *testing.T) {
	seedPrometheusCatalog(t)

	db, err := data.OpenDatabase(false)
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	store.Put(promconfig.KVNamespace, "scrape_jobs", "api", 0)
	store.Put(promconfig.KVNamespace, "scrape/api", `[{"job_name":"api","static_configs":[{"targets":["127.0.0.1:9090"]}]}]`, 0)
	require.NoError(t, kv.PersistToTransaction(tx, store))
	require.NoError(t, tx.Commit())

	require.NoError(t, PrometheusScrape(""))
	assert.Error(t, PrometheusScrape("web"))
}
