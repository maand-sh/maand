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

func TestRunbookSlugFromPath(t *testing.T) {
	job, slug, ok := RunbookSlugFromPath("api/_prometheus/runbooks/ApiDown.md")
	require.True(t, ok)
	assert.Equal(t, "api", job)
	assert.Equal(t, "ApiDown", slug)
}
