package cat

import (
	"crypto/x509/pkix"
	"database/sql"
	"os"
	"path"
	"testing"
	"time"

	"maand/bucket"
	"maand/build"
	"maand/data"
	"maand/initialize"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func TestParseJobWorkerCertNamespace(t *testing.T) {
	job, worker, ok := parseJobWorkerCertNamespace("maand/job/api/worker/10.0.0.1")
	require.True(t, ok)
	assert.Equal(t, "api", job)
	assert.Equal(t, "10.0.0.1", worker)

	_, _, ok = parseJobWorkerCertNamespace("vars/job/api")
	assert.False(t, ok)
}

func TestCertExpiryStatus(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	assert.Equal(t, certStatusExpired, certExpiryStatus(now.Add(-24*time.Hour), 30, now))
	assert.Equal(t, certStatusExpiring, certExpiryStatus(now.Add(10*24*time.Hour), 30, now))
	assert.Equal(t, certStatusOK, certExpiryStatus(now.Add(90*24*time.Hour), 30, now))
}

func TestCerts_listsCAAndJobCertificates(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), 0o644))
	require.NoError(t, initialize.Execute())

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "api")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["worker"],
		"certs": {"tls": {"subject": {"common_name": "api.internal"}}}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(""), 0o644))

	require.NoError(t, build.Execute())

	stdout := captureStdout(t, func() {
		require.NoError(t, Certs("", ""))
	})
	assert.Contains(t, stdout, "ca")
	assert.Contains(t, stdout, "api")
	assert.Contains(t, stdout, "10.0.0.1")
	assert.Contains(t, stdout, "tls")
	assert.Contains(t, stdout, "ok")
}

func TestCerts_filtersByJob(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{"host":"10.0.0.1"}]`), 0o644))
	require.NoError(t, initialize.Execute())

	for _, jobName := range []string{"api", "web"} {
		jobDir := path.Join(bucket.WorkspaceLocation, "jobs", jobName)
		require.NoError(t, os.MkdirAll(jobDir, 0o755))
		require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
			"selectors": ["worker"],
			"certs": {"tls": {"subject": {"common_name": "`+jobName+`.internal"}}}
		}`), 0o644))
		require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(""), 0o644))
	}

	require.NoError(t, build.Execute())

	stdout := captureStdout(t, func() {
		require.NoError(t, Certs("api", ""))
	})
	assert.Contains(t, stdout, "api")
	assert.NotContains(t, stdout, "web.internal")
}

func TestCerts_notFoundWithoutJobCerts(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, data.MigrateSchema(tx))
	require.NoError(t, tx.Commit())

	orig := bucket.Location
	bucket.Location = t.TempDir()
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	err = Certs("", "")
	assert.Error(t, err)
}

func TestCertEntryFromPEM(t *testing.T) {
	t.Cleanup(kv.ResetStoreForTest)
	dir := t.TempDir()
	bucket.Location = dir
	bucket.UpdatePath()
	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte("[]"), 0o644))
	require.NoError(t, initialize.Execute())

	certDir := t.TempDir()
	require.NoError(t, build.GenerateCert(certDir, "tls", pkix.Name{CommonName: "test.local"}, []string{"10.0.0.1"}, 365, true))
	pemBytes, err := os.ReadFile(path.Join(certDir, "tls.crt"))
	require.NoError(t, err)

	now := time.Now().UTC()
	entry, err := certEntryFromPEM("job", "api", "10.0.0.1", "tls", pemBytes, 30, now)
	require.NoError(t, err)
	assert.Equal(t, "test.local", entry.commonName)
	assert.Equal(t, certStatusOK, entry.status)
	assert.Greater(t, entry.daysLeft, 300)
}
