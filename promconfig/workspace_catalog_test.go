package promconfig

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/data"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListWorkspacePrometheusSummaries(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "keeper")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml"), []byte("- job_name: keeper\n"), 0o644))

	summaries, err := ListWorkspacePrometheusSummaries(nil)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, "keeper", summaries[0].Job)
	assert.True(t, summaries[0].Scrape)
}

func TestReadWorkspacePrometheusFile_shorthand(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "keeper")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "scrape.yaml"), []byte("scrape-content\n"), 0o644))

	content, err := ReadWorkspacePrometheusFile("keeper", "scrape")
	require.NoError(t, err)
	assert.Equal(t, "scrape-content\n", content)
}

func TestNormalizePrometheusRelPath(t *testing.T) {
	assert.Equal(t, ScrapeFileName, NormalizePrometheusRelPath("scrape"))
	assert.Equal(t, ScrapeFileName, NormalizePrometheusRelPath(""))
	assert.Equal(t, "alerts/slo.yaml", NormalizePrometheusRelPath("alerts/slo.yaml"))
}

func TestMergePrometheusSummaries(t *testing.T) {
	merged := MergePrometheusSummaries(
		[]data.PrometheusJobSummary{{Job: "api", Scrape: true, Alerts: 1}},
		[]data.PrometheusJobSummary{{Job: "api", Alerts: 2, Runbooks: 1}},
	)
	require.Len(t, merged, 1)
	assert.True(t, merged[0].Scrape)
	assert.Equal(t, 2, merged[0].Alerts)
	assert.Equal(t, 1, merged[0].Runbooks)
}
