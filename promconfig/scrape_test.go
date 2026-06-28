package promconfig

import (
	"testing"

	"maand/workspace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScrapeConfigsRequireActiveAllocations(t *testing.T) {
	configs, err := ParseScrapeFile([]byte(`
- job_name: api
  static_configs:
    - targets:
        - maand:port/api_metrics_port
`))
	require.NoError(t, err)
	assert.True(t, ScrapeConfigsRequireActiveAllocations(configs))

	literal, err := ParseScrapeFile([]byte(`
- job_name: external
  static_configs:
    - targets:
        - blackbox.internal:9115
`))
	require.NoError(t, err)
	assert.False(t, ScrapeConfigsRequireActiveAllocations(literal))
}

func TestExpandScrapeConfigs_noActiveAllocations(t *testing.T) {
	configs, err := ParseScrapeFile([]byte(`
- job_name: api
  static_configs:
    - targets:
        - maand:port/api_metrics_port
`))
	require.NoError(t, err)

	ports := workspace.ManifestPorts{"api_metrics_port": {}}
	_, err = ExpandScrapeConfigs("api", configs, ports, map[string]int{"api_metrics_port": 30421}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoActiveScrapeTargets)
}

func TestResolveMetricsPortKey(t *testing.T) {
	ports := workspace.ManifestPorts{
		"api_metrics_port": {},
	}
	key, err := ResolveMetricsPortKey("api", ports)
	require.NoError(t, err)
	assert.Equal(t, "api_metrics_port", key)

	ports = workspace.ManifestPorts{"metrics_port": {}}
	key, err = ResolveMetricsPortKey("worker", ports)
	require.NoError(t, err)
	assert.Equal(t, "metrics_port", key)

	ports = workspace.ManifestPorts{
		"a_metrics_port": {},
		"b_metrics_port": {},
	}
	_, err = ResolveMetricsPortKey("worker", ports)
	assert.Error(t, err)
}

func TestExpandScrapeConfigs(t *testing.T) {
	configs, err := ParseScrapeFile([]byte(`
- job_name: api
  metrics_path: /metrics
  static_configs:
    - targets:
        - maand:port/api_metrics_port
      labels:
        maand_job: api
`))
	require.NoError(t, err)

	ports := workspace.ManifestPorts{"api_metrics_port": {}}
	expanded, err := ExpandScrapeConfigs("api", configs, ports, map[string]int{"api_metrics_port": 30421}, []string{"10.0.0.1", "10.0.0.2"})
	require.NoError(t, err)
	require.Len(t, expanded, 1)

	static := expanded[0]["static_configs"].([]interface{})
	block := static[0].(map[string]interface{})
	targets := block["targets"].([]interface{})
	assert.Equal(t, "10.0.0.1:30421", targets[0])
	assert.Equal(t, "10.0.0.2:30421", targets[1])
}

func TestExpandScrapeConfigs_maandJobPlaceholder(t *testing.T) {
	configs, err := ParseScrapeFile([]byte(`
- job_name: maand:job
  static_configs:
    - targets:
        - 127.0.0.1:9090
      labels:
        maand_job: maand:job
`))
	require.NoError(t, err)

	expanded, err := ExpandScrapeConfigs("clickhouse_keeper", configs, nil, nil, []string{"10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "clickhouse_keeper", expanded[0]["job_name"])

	static := expanded[0]["static_configs"].([]interface{})
	block := static[0].(map[string]interface{})
	labels := block["labels"].(map[string]interface{})
	assert.Equal(t, "clickhouse_keeper", labels["maand_job"])
}

func TestResolveScrapeJobName(t *testing.T) {
	assert.Equal(t, "api", ResolveScrapeJobName("api", "api"))
	assert.Equal(t, "api", ResolveScrapeJobName("api", JobPlaceholder))
}

func TestExpandScrapeConfigs_literalTarget(t *testing.T) {
	configs, err := ParseScrapeFile([]byte(`
- job_name: external
  static_configs:
    - targets:
        - blackbox.internal:9115
`))
	require.NoError(t, err)

	expanded, err := ExpandScrapeConfigs("api", configs, nil, nil, nil)
	require.NoError(t, err)
	static := expanded[0]["static_configs"].([]interface{})
	block := static[0].(map[string]interface{})
	targets := block["targets"].([]interface{})
	assert.Equal(t, "blackbox.internal:9115", targets[0])
}

func TestScrapeConfigsYAML(t *testing.T) {
	configs, err := ParseScrapeFile([]byte(`
- job_name: api
  static_configs:
    - targets:
        - 10.0.0.1:9090
`))
	require.NoError(t, err)
	yamlFragment, err := ScrapeConfigsYAML(configs)
	require.NoError(t, err)
	assert.Contains(t, yamlFragment, "job_name: api")
	assert.True(t, len(yamlFragment) > 0)
	assert.Equal(t, '\n', rune(yamlFragment[0]))
}

func TestValidateScrapeConfigs_rejectsSD(t *testing.T) {
	configs := []map[string]interface{}{
		{
			"job_name":              "api",
			"kubernetes_sd_configs": []interface{}{map[string]interface{}{}},
		},
	}
	err := ValidateScrapeConfigs("api", configs)
	assert.Error(t, err)
}

func TestValidateScrapePortReferences(t *testing.T) {
	configs, err := ParseScrapeFile([]byte(`
- job_name: api
  static_configs:
    - targets:
        - maand:port/api_metrics_port
`))
	require.NoError(t, err)

	ports := workspace.ManifestPorts{"api_metrics_port": {}}
	assert.NoError(t, ValidateScrapePortReferences("api", configs, ports))

	ports = workspace.ManifestPorts{}
	assert.Error(t, ValidateScrapePortReferences("api", configs, ports))
}
