package promconfig

import (
	"testing"

	"maand/data"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderRuleFilesYAML(t *testing.T) {
	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	tx, err := db.Begin()
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	_, err = tx.Exec(`
		INSERT INTO job (job_id, name) VALUES ('job-api', 'api'), ('job-web', 'web');
		INSERT INTO job_files (job_id, path, content, isdir) VALUES
			('job-api', 'api/_prometheus/alerts/slo.yaml', 'groups: []', 0),
			('job-web', 'web/_prometheus/alerts/errors.yaml', 'groups: []', 0);
	`)
	require.NoError(t, err)

	yamlFragment, err := RenderRuleFilesYAML(tx)
	require.NoError(t, err)
	assert.Contains(t, yamlFragment, "rule_files:")
	assert.Contains(t, yamlFragment, "  - rules/api/slo.yaml")
	assert.Contains(t, yamlFragment, "  - rules/web/errors.yaml")
}

func TestRenderRuleFilesYAML_empty(t *testing.T) {
	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	tx, err := db.Begin()
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	yamlFragment, err := RenderRuleFilesYAML(tx)
	require.NoError(t, err)
	assert.Empty(t, yamlFragment)
}
