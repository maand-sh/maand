package tests

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strconv"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/healthcheck"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupJobWithProvisionedPortHealthCheck(t *testing.T, healthChecksJSON string) (assignedPort int) {
	t.Helper()
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"127.0.0.1","labels":["app"]}]`)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "app")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	manifest := `{
		"selectors": ["app"],
		"resources": {"ports": {"api_port": {}}},
		"health_check": {"checks": ` + healthChecksJSON + `}
	}`
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))
	executeBuild(t)

	MustQueryRow(t, `SELECT port FROM job_ports WHERE name = 'api_port'`, &assignedPort)
	return assignedPort
}

func openHealthCheckSession(t *testing.T) (*sql.Tx, *bucket.Runtime, func()) {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	cancel, err := healthcheck.PrepareRuntime(tx)
	require.NoError(t, err)

	bucketID, err := data.GetBucketID(tx)
	require.NoError(t, err)
	rt, err := bucket.SetupRuntime(bucketID, bucket.NewRunContext("test", 0))
	require.NoError(t, err)

	cleanup := func() {
		_ = rt.Stop()
		cancel()
		_ = tx.Rollback()
		_ = db.Close()
	}
	return tx, rt, cleanup
}

func TestBuildRejectsInvalidHealthCheckPort(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1","labels":["app"]}]`)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "app")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["app"],
		"resources": {"ports": {"api_port": {}}},
		"health_check": {"checks": [{"type": "tcp", "port": "wrong_port"}]}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrInvalidManifest)
}

func TestManifestTCPHealthCheck(t *testing.T) {
	port := setupJobWithProvisionedPortHealthCheck(t, `[{"type": "tcp", "port": "api_port"}]`)

	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	tx, rt, cleanup := openHealthCheckSession(t)
	defer cleanup()

	_, err = healthcheck.HealthCheck(tx, rt, false, false, "app", false)
	require.NoError(t, err)
}

func TestBuildAllowsManifestAndCommandHealthCheck(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"127.0.0.1","labels":["app"]}]`)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "app")
	require.NoError(t, os.MkdirAll(path.Join(jobDir, "_modules"), 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "_modules", "command_health.py"), []byte(`import sys; sys.exit(0)`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["app"],
		"resources": {"ports": {"api_port": 30010}},
		"health_check": {"checks": [{"type": "tcp", "port": "api_port"}]},
		"commands": {"command_health": {"executed_on": ["health_check"]}}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))

	executeBuild(t)

	var stored string
	MustQueryRow(t, `SELECT health_check FROM job WHERE name = 'app'`, &stored)
	require.NotEmpty(t, stored)

	count := MustQueryCount(t,
		`SELECT count(*) FROM job_commands WHERE job = 'app' AND executed_on = 'health_check'`,
	)
	assert.Equal(t, 1, count)

	var assignedPort int
	MustQueryRow(t, `SELECT port FROM job_ports WHERE name = 'api_port'`, &assignedPort)

	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(assignedPort)))
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	tx, rt, cleanup := openHealthCheckSession(t)
	defer cleanup()

	_, err = healthcheck.HealthCheck(tx, rt, false, false, "app", false)
	require.NoError(t, err)
}

func TestBuildRejectsSSHHealthCheckWithoutCommand(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1","labels":["app"]}]`)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "app")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["app"],
		"health_check": {"checks": [{"type": "ssh"}]}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrInvalidManifest)
}

func TestBuildStoresHealthCheckJSON(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1","labels":["app"]}]`)

	jobDir := path.Join(bucket.WorkspaceLocation, "jobs", "app")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "manifest.json"), []byte(`{
		"selectors": ["app"],
		"resources": {"ports": {"api_port": 30010}},
		"health_check": {
			"checks": [
				{"type": "tcp", "port": "api_port"},
				{"type": "ssh", "command": "true"}
			]
		}
	}`), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobDir, "Makefile"), []byte(Makefile()), 0o644))

	executeBuild(t)

	var stored string
	MustQueryRow(t, `SELECT health_check FROM job WHERE name = 'app'`, &stored)
	require.NotEmpty(t, stored)

	var decoded struct {
		Checks []struct {
			Type    string `json:"type"`
			Port    string `json:"port"`
			Command string `json:"command"`
		} `json:"checks"`
	}
	require.NoError(t, json.Unmarshal([]byte(stored), &decoded))
	require.Len(t, decoded.Checks, 2)
	assert.Equal(t, "tcp", decoded.Checks[0].Type)
	assert.Equal(t, "ssh", decoded.Checks[1].Type)
	assert.Equal(t, "true", decoded.Checks[1].Command)
}

func TestManifestHTTPHealthCheck(t *testing.T) {
	port := setupJobWithProvisionedPortHealthCheck(t, `[{"type": "http", "port": "api_port", "path": "/ready"}]`)

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	require.NoError(t, err)
	server.Listener = listener
	server.Start()
	defer server.Close()

	tx, rt, cleanup := openHealthCheckSession(t)
	defer cleanup()

	_, err = healthcheck.HealthCheck(tx, rt, false, false, "app", false)
	require.NoError(t, err)
}

func TestHealthCheckSkipsJobWithNoManifestOrCommands(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1","labels":["plain"]}]`)
	writeMinimalJob(t, "plain", `{"selectors":["plain"]}`)
	runBuild(t)

	tx, rt, cleanup := openHealthCheckSession(t)
	defer cleanup()

	_, err := healthcheck.HealthCheck(tx, rt, false, false, "plain", false)
	require.NoError(t, err)
}

func TestCheckWorkersUsesConfiguredSSHPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	initFreshBucket(t)
	require.NoError(t, os.WriteFile(path.Join(bucket.Location, "maand.conf"), []byte(
		"ssh_user = \"agent\"\nssh_key = \"worker.key\"\nssh_port = "+portStr+"\n",
	), 0o644))
	writeWorkersJSON(t, `[{"host":"127.0.0.1","labels":["w1"]}]`)
	runBuild(t)

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	require.NoError(t, healthcheck.CheckWorkers(tx, false))
}
