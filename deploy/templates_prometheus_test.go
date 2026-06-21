package deploy

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"maand/kv"
	"maand/promconfig"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranspile_scrapeConfigsTemplate(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "prometheus", "10.0.0.1", 0)
	apiJobID := env.insertJob(t, tx, "api", 0, 1)
	env.insertAllocation(t, tx, "alloc-api-10.0.0.1", "10.0.0.1", "api", 0, 0, 0)
	env.insertJobFile(t, tx, apiJobID, path.Join("api", "manifest.json"), `{
		"selectors": ["worker"],
		"resources": {"ports": {"api_metrics_port": {}}}
	}`, false)
	_, err := tx.Exec(`INSERT INTO job_ports (job_id, name, port) VALUES (?, 'api_metrics_port', 30421)`, apiJobID)
	require.NoError(t, err)

	jobID := "job-prometheus"
	tpl := `global:
  scrape_interval: 15s

scrape_configs:
  - job_name: prometheus
    static_configs:
      - targets: ['127.0.0.1:9090']
{{ scrapeConfigs }}
`
	env.insertJobFile(t, tx, jobID, path.Join("prometheus", "prometheus.yml.tpl"), tpl, false)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	unexpanded, err := json.Marshal([]map[string]interface{}{
		{
			"job_name": "api",
			"static_configs": []interface{}{
				map[string]interface{}{
					"targets": []interface{}{"maand:port/api_metrics_port"},
				},
			},
		},
	})
	require.NoError(t, err)
	store.Put(promconfig.KVNamespace, "scrape_jobs", "api", 0)
	store.Put(promconfig.KVNamespace, "scrape/api", string(unexpanded), 0)
	require.NoError(t, prepareJobsFiles(tx, []string{"prometheus"}))
	require.NoError(t, transpile(tx, "prometheus", "10.0.0.1"))
	require.NoError(t, tx.Rollback())

	out := path.Join(env.root, "tmp", "workers", "10.0.0.1", "jobs", "prometheus", "prometheus.yml")
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	text := string(content)
	assert.Contains(t, text, "job_name: prometheus")
	assert.Contains(t, text, "job_name: api")
	assert.Contains(t, text, "10.0.0.1:30421")
}

func TestTranspile_ruleFilesTemplate(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "prometheus", "10.0.0.1", 0)
	apiJobID := env.insertJob(t, tx, "api", 0, 1)
	env.insertJobFile(t, tx, apiJobID, path.Join("api", "_prometheus", "alerts", "slo.yaml"), "groups: []\n", false)

	tpl := `global:
  scrape_interval: 15s
{{ ruleFiles }}
`
	env.insertJobFile(t, tx, "job-prometheus", path.Join("prometheus", "prometheus.yml.tpl"), tpl, false)
	require.NoError(t, prepareJobsFiles(tx, []string{"prometheus"}))
	require.NoError(t, transpile(tx, "prometheus", "10.0.0.1"))
	require.NoError(t, tx.Rollback())

	out := path.Join(env.root, "tmp", "workers", "10.0.0.1", "jobs", "prometheus", "prometheus.yml")
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	text := string(content)
	assert.Contains(t, text, "rule_files:")
	assert.Contains(t, text, "  - rules/api/slo.yaml")
}

func TestAssemblePrometheusAlertRules(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "prometheus", "10.0.0.1", 0)
	apiJobID := env.insertJob(t, tx, "api", 0, 1)
	env.insertJobFile(t, tx, apiJobID, path.Join("api", "_prometheus", "alerts", "slo.yaml"), "groups: []\n", false)
	require.NoError(t, prepareJobsFiles(tx, []string{"prometheus"}))
	dest := path.Join(env.root, "tmp", "workers", "10.0.0.1", "jobs", "prometheus")
	require.NoError(t, assemblePrometheusAlertRules(tx, dest))
	require.NoError(t, tx.Rollback())

	out := path.Join(dest, "rules", "api", "slo.yaml")
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "groups: []\n", string(content))
}
