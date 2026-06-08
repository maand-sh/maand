package healthcheck

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"maand/workspace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() {
		_ = ln.Close()
	}()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	port, err := net.LookupPort("tcp", portStr)
	require.NoError(t, err)

	require.NoError(t, probeTCP("127.0.0.1", port, time.Second))
	assert.Error(t, probeTCP("127.0.0.1", port+1, 100*time.Millisecond))
}

func TestProbeHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	host, portStr, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	port, err := net.LookupPort("tcp", portStr)
	require.NoError(t, err)

	require.NoError(t, probeHTTP(host, port, workspace.HealthCheckProbe{Path: "/health"}, time.Second))
	err = probeHTTP(host, port, workspace.HealthCheckProbe{Path: "/missing", ExpectStatus: 200}, time.Second)
	assert.Error(t, err)
}

func TestProbeSSHEmptyCommand(t *testing.T) {
	err := probeSSH("10.0.0.1", "  ", time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty command")
}
