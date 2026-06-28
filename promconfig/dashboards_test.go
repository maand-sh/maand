package promconfig

import (
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardRelFromPath(t *testing.T) {
	job, rel, ok := DashboardRelFromPath("api/_prometheus/dashboards/overview.html")
	assert.True(t, ok)
	assert.Equal(t, "api", job)
	assert.Equal(t, "overview.html", rel)

	job, rel, ok = DashboardRelFromPath("api/_prometheus/dashboards/slo/latency.html")
	assert.True(t, ok)
	assert.Equal(t, "api", job)
	assert.Equal(t, "slo/latency.html", rel)

	_, _, ok = DashboardRelFromPath("api/_prometheus/runbooks/ApiDown.md")
	assert.False(t, ok)
}

func TestListDashboardFiles(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api", "_prometheus", "dashboards", "slo")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "latency.html"), []byte("<html></html>"), 0o644))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "jobs", "api", "_prometheus", "dashboards", "overview.html"), []byte("<html></html>"), 0o644))

	entries, err := ListDashboardFiles("api")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "/consoles/dashboards/api/overview.html", DashboardConsolePath("api", entries[0].Rel))
}
