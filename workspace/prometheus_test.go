package workspace

import (
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePrometheusServerFiles_mutualExclusion(t *testing.T) {
	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "prom")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "prometheus.yml"), []byte("global: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "prometheus.yml.tpl"), []byte("global: {}\n"), 0o644))
	t.Cleanup(func() {
		_ = os.RemoveAll(jobDir)
	})

	assert.ErrorIs(t, ValidatePrometheusServerFiles("prom"), bucket.ErrInvalidJob)
}

func TestValidatePrometheusServerFiles_singleFileOK(t *testing.T) {
	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "prom2")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "prometheus.yml.tpl"), []byte("global: {}\n"), 0o644))
	t.Cleanup(func() {
		_ = os.RemoveAll(jobDir)
	})

	assert.NoError(t, ValidatePrometheusServerFiles("prom2"))
}
