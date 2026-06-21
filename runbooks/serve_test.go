package runbooks

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/data"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeRunbook(t *testing.T) {
	root := t.TempDir()
	oldLocation := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = oldLocation
		bucket.UpdatePath()
	})

	require.NoError(t, os.MkdirAll(path.Join(root, "data"), 0o755))
	require.NoError(t, initialize.Execute())

	writeWorkers := `[{"host":"10.0.0.1"}]`
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(writeWorkers), 0o644))

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_prometheus", "runbooks"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(".PHONY: start stop restart\nstart:\nstop:\nrestart:\n"), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_prometheus", "runbooks", "ApiDown.md"), []byte("# Api Down\n"), 0o644))
	require.NoError(t, build.Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{job}/{slug}", func(w http.ResponseWriter, r *http.Request) {
		serveRunbook(w, r, db, false)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/ApiDown", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), "<h1>Api Down</h1>")
}
