package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/promconfig"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePrometheusServerJob(t *testing.T) {
	t.Helper()
	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "prometheus")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["worker"],
		"resources": {"ports": {"prometheus_http_port": {}}}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "prometheus.yml.tpl"), []byte(`global:
  scrape_interval: 15s
scrape_configs:
{{ scrapeConfigs }}`), 0o644))
}

func TestBuildPrometheusCatalog_scrapeAndKV(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writePrometheusServerJob(t)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus", "alerts"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["worker"],
		"resources": {"ports": {"api_metrics_port": {}}}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml"), []byte(`
- job_name: api
  metrics_path: /metrics
  static_configs:
    - targets:
        - maand:port/_implicit
`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "alerts", "slo.yaml"), []byte(`
groups:
  - name: api
    rules:
      - alert: ApiDown
        expr: up == 0
`), 0o644))

	runBuild(t)

	scrapeJobs, err := GetKey(promconfig.KVNamespace, "scrape_jobs")
	require.NoError(t, err)
	assert.Equal(t, "api", scrapeJobs)

	scrapeConfigs, err := GetKey(promconfig.KVNamespace, "scrape_configs")
	require.NoError(t, err)
	assert.Contains(t, scrapeConfigs, "maand:port/_implicit")
	assert.NotContains(t, scrapeConfigs, "10.0.0.1:")

	jobName, err := GetKey("maand/job/api", "job_name")
	require.NoError(t, err)
	assert.Equal(t, "api", jobName)

	_, err = GetKey(promconfig.KVNamespace, "scrape_configs_yaml")
	assert.Error(t, err)

	alertJobs, err := GetKey(promconfig.KVNamespace, "alert_jobs")
	require.NoError(t, err)
	assert.Equal(t, "api", alertJobs)
}

func TestBuildPrometheusCatalog_scrapeWithoutAllocations(t *testing.T) {
	initFreshBucket(t)
	writePrometheusServerJob(t)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["worker"],
		"resources": {"ports": {"api_metrics_port": {}}}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml"), []byte(`
- job_name: api
  static_configs:
    - targets:
        - maand:port/api_metrics_port
`), 0o644))

	runBuild(t)

	scrapeConfigs, err := GetKey(promconfig.KVNamespace, "scrape_configs")
	require.NoError(t, err)
	assert.Contains(t, scrapeConfigs, "maand:port/api_metrics_port")
}

func TestBuildPrometheusCatalog_scrapeTemplate(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writePrometheusServerJob(t)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["worker"],
		"resources": {"ports": {"api_metrics_port": {}}}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml.tpl"), []byte(`
- job_name: {{ .Job }}
  static_configs:
    - targets:
        - maand:port/api_metrics_port
`), 0o644))

	runBuild(t)

	perJob, err := GetKey(promconfig.KVNamespace, "scrape/api")
	require.NoError(t, err)
	assert.Contains(t, perJob, `"job_name":"api"`)
	assert.Contains(t, perJob, "maand:port/api_metrics_port")
}

func TestBuildPrometheusCatalog_scrapeTemplateMutualExclusion(t *testing.T) {
	initFreshBucket(t)
	writePrometheusServerJob(t)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml"), []byte("- job_name: api\n"), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml.tpl"), []byte("- job_name: api\n"), 0o644))

	err := build.Execute()
	assert.Error(t, err)
}

func TestBuildPrometheusCatalog_optionalWithoutPrometheusJob(t *testing.T) {
	initFreshBucket(t)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	// Invalid combination must be ignored when no Prometheus server job exists.
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml"), []byte("- job_name: api\n"), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml.tpl"), []byte("- job_name: api\n"), 0o644))

	require.NoError(t, build.Execute())
}

func TestBuildPrometheusCatalog_maandJobPlaceholder(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writePrometheusServerJob(t)

	for _, jobName := range []string{"keeper_a", "keeper_b"} {
		jobDir := path.Join(bucket.WorkspaceLocation, "jobs", jobName)
		require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus"), 0o755))
		require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
			"selectors": ["worker"],
			"resources": {"ports": {"metrics_port": {}}}
		}`), 0o644))
		require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
		require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml"), []byte(`
- job_name: maand:job
  static_configs:
    - targets:
        - maand:port/metrics_port
`), 0o644))
	}

	runBuild(t)

	for _, jobName := range []string{"keeper_a", "keeper_b"} {
		value, err := GetKey("maand/job/"+jobName, "job_name")
		require.NoError(t, err)
		assert.Equal(t, jobName, value)
	}
}

func TestBuildPrometheusCatalog_duplicateJobNameFails(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writePrometheusServerJob(t)

	for _, jobName := range []string{"a", "b"} {
		jobDir := path.Join(bucket.WorkspaceLocation, "jobs", jobName)
		require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus"), 0o755))
		require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
			"selectors": ["worker"],
			"resources": {"ports": {"metrics_port": {}}}
		}`), 0o644))
		require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
		require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml"), []byte(`
- job_name: same
  static_configs:
    - targets:
        - maand:port/metrics_port
`), 0o644))
	}

	err := build.Execute()
	assert.Error(t, err)
}

func TestBuildPrometheusCatalog_prometheusConfigMutualExclusion(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "prometheus")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "prometheus.yml"), []byte("global: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "prometheus.yml.tpl"), []byte("global: {}\n"), 0o644))

	err := build.Execute()
	assert.Error(t, err)
}

func TestBuildPrometheusCatalog_runbookValidation(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writePrometheusServerJob(t)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus", "alerts"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "alerts", "slo.yaml"), []byte(`
groups:
  - name: api
    rules:
      - alert: ApiDown
        expr: up == 0
        annotations:
          runbook: Missing
`), 0o644))

	err := build.Execute()
	assert.Error(t, err)
}

func TestBuildPrometheusCatalog_duplicateAlertNameFails(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writePrometheusServerJob(t)

	for _, jobName := range []string{"a", "b"} {
		jobDir := path.Join(bucket.WorkspaceLocation, "jobs", jobName)
		require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus", "alerts"), 0o755))
		require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
		require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
		require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "alerts", "slo.yaml"), []byte(`
groups:
  - name: shared
    rules:
      - alert: SameAlert
        expr: up == 0
`), 0o644))
	}

	err := build.Execute()
	assert.Error(t, err)
}

func TestBuildPrometheusCatalog_recordingRuleAllowed(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writePrometheusServerJob(t)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus", "alerts"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "alerts", "rec.yaml"), []byte(`
groups:
  - name: api
    rules:
      - record: api:up:sum
        expr: sum(up{job="api"})
`), 0o644))

	runBuild(t)
}

func TestBuildPrometheusCatalog_missingExprFails(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writePrometheusServerJob(t)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus", "alerts"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "alerts", "slo.yaml"), []byte(`
groups:
  - name: api
    rules:
      - alert: ApiDown
`), 0o644))

	err := build.Execute()
	assert.Error(t, err)
}
