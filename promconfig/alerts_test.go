package promconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAlertFile_runbookReference(t *testing.T) {
	data := []byte(`
groups:
  - name: api
    rules:
      - alert: ApiDown
        expr: up == 0
        annotations:
          runbook: ApiDown
`)
	err := ValidateAlertFile("api", "alerts/slo.yaml", data, map[string]struct{}{"ApiDown": {}})
	assert.NoError(t, err)

	err = ValidateAlertFile("api", "alerts/slo.yaml", data, map[string]struct{}{})
	assert.Error(t, err)
}

func TestValidateAlertFile_recordingRule(t *testing.T) {
	data := []byte(`
groups:
  - name: api
    rules:
      - record: api:up:sum
        expr: sum(up{job="api"})
`)
	err := ValidateAlertFile("api", "alerts/rec.yaml", data, map[string]struct{}{})
	assert.NoError(t, err)
}

func TestValidateAlertFile_alertAndRecordFails(t *testing.T) {
	data := []byte(`
groups:
  - name: api
    rules:
      - alert: ApiDown
        record: api:down
        expr: up == 0
`)
	err := ValidateAlertFile("api", "alerts/slo.yaml", data, map[string]struct{}{})
	assert.Error(t, err)
}

func TestValidateAlertFile_missingExprFails(t *testing.T) {
	data := []byte(`
groups:
  - name: api
    rules:
      - alert: ApiDown
`)
	err := ValidateAlertFile("api", "alerts/slo.yaml", data, map[string]struct{}{})
	assert.Error(t, err)
}

func TestAlertRelPath(t *testing.T) {
	rel, ok := AlertRelPath("api/_prometheus/alerts/slo.yaml")
	require.True(t, ok)
	assert.Equal(t, "alerts/slo.yaml", rel)
}

func TestInjectRunbookURLs(t *testing.T) {
	data := []byte(`
groups:
  - name: api
    rules:
      - alert: ApiDown
        expr: up == 0
        annotations:
          runbook: ApiDown
      - alert: ApiSlow
        expr: latency > 1
        annotations:
          runbook: ApiSlow
          runbook_url: http://custom.example/runbook
      - record: api:up:sum
        expr: sum(up{job="api"})
`)
	out, err := InjectRunbookURLs("api", data, "http://10.0.0.1:9090")
	require.NoError(t, err)
	text := string(out)
	wantURL := "http://10.0.0.1:9090/consoles/runbooks/api/ApiDown.html"
	assert.Contains(t, text, "runbook_url: "+wantURL)
	assert.NotContains(t, text, "runbook:")
	assert.NotContains(t, text, "Runbook:")
	assert.Contains(t, text, "runbook_url: http://custom.example/runbook")
	assert.NotContains(t, text, "runbook: ApiSlow")
}

func TestRunbookConsolePath(t *testing.T) {
	assert.Equal(t, "/consoles/runbooks/node_agent/container_restarting.html",
		RunbookConsolePath("node_agent", "container_restarting"))
}

func TestRunbookSlugFromPath(t *testing.T) {
	job, slug, ok := RunbookSlugFromPath("api/_prometheus/runbooks/ApiDown.md")
	require.True(t, ok)
	assert.Equal(t, "api", job)
	assert.Equal(t, "ApiDown", slug)
}
