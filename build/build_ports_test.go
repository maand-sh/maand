package build

import (
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/workspace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortAllocatorStableAndReuse(t *testing.T) {
	existing := data.JobPortAssignments{
		"a": {"web_port": 30001},
	}
	alloc := newPortAllocator(existing, bucket.PortRange{Min: 30000, Max: 30002})

	port, err := alloc.assignProvisioned("a", "web_port")
	require.NoError(t, err)
	assert.Equal(t, 30001, port)

	port, err = alloc.assignProvisioned("b", "api_port")
	require.NoError(t, err)
	assert.Equal(t, 30000, port)

	port, err = alloc.assignProvisioned("b", "api_port")
	require.NoError(t, err)
	assert.Equal(t, 30000, port)
}

func TestPortAllocatorReleaseRemoved(t *testing.T) {
	existing := data.JobPortAssignments{
		"a": {"web_port": 30000, "admin_port": 30001},
	}
	alloc := newPortAllocator(existing, bucket.PortRange{Min: 30000, Max: 30001})

	alloc.releaseRemoved("a", []string{"admin_port"})

	port, err := alloc.assignProvisioned("b", "other_port")
	require.NoError(t, err)
	assert.Equal(t, 30000, port)
}

func TestPortAllocatorExhausted(t *testing.T) {
	existing := data.JobPortAssignments{
		"a": {"p1": 30000},
	}
	alloc := newPortAllocator(existing, bucket.PortRange{Min: 30000, Max: 30000})

	_, err := alloc.assignProvisioned("b", "p2")
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrPortRangeExhausted)
}

func TestPortAllocatorStableAcrossMultipleJobs(t *testing.T) {
	existing := data.JobPortAssignments{
		"api":      {"http_port": 30001},
		"database": {"db_port": 30002},
	}
	alloc := newPortAllocator(existing, bucket.PortRange{Min: 30000, Max: 30005})

	for _, tc := range []struct {
		job, name string
		want      int
	}{
		{"api", "http_port", 30001},
		{"database", "db_port", 30002},
		{"api", "http_port", 30001},
		{"database", "db_port", 30002},
	} {
		port, err := alloc.assignProvisioned(tc.job, tc.name)
		require.NoError(t, err, "%s/%s", tc.job, tc.name)
		assert.Equal(t, tc.want, port)
	}

	port, err := alloc.assignProvisioned("worker", "metrics_port")
	require.NoError(t, err)
	assert.Equal(t, 30000, port)

	port, err = alloc.assignProvisioned("api", "http_port")
	require.NoError(t, err)
	assert.Equal(t, 30001, port)
}

func TestPortAllocatorFixedPort(t *testing.T) {
	alloc := newPortAllocator(nil, bucket.PortRange{Min: 30000, Max: 30005})

	port, err := alloc.assignFixed("api", "http_port", 30001)
	require.NoError(t, err)
	assert.Equal(t, 30001, port)

	port, err = alloc.assignFixed("api", "http_port", 30001)
	require.NoError(t, err)
	assert.Equal(t, 30001, port)

	_, err = alloc.assignFixed("db", "db_port", 30001)
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrPortCollision)

	port, err = alloc.assignProvisioned("worker", "metrics_port")
	require.NoError(t, err)
	assert.Equal(t, 30000, port)
}

func TestPortAllocatorFixedPortChangeReleasesPrevious(t *testing.T) {
	existing := data.JobPortAssignments{
		"api": {"http_port": 30001},
	}
	alloc := newPortAllocator(existing, bucket.PortRange{Min: 30000, Max: 30005})

	port, err := alloc.assignFixed("api", "http_port", 30002)
	require.NoError(t, err)
	assert.Equal(t, 30002, port)

	port, err = alloc.assignFixed("worker", "metrics_port", 30001)
	require.NoError(t, err)
	assert.Equal(t, 30001, port)
}

func TestPortAllocatorResolveProvisionedAndFixed(t *testing.T) {
	alloc := newPortAllocator(nil, bucket.PortRange{Min: 30000, Max: 30005})

	port, err := alloc.resolve("api", "http_port", workspace.ManifestPortBinding{Fixed: intPtr(30001)})
	require.NoError(t, err)
	assert.Equal(t, 30001, port)

	port, err = alloc.resolve("worker", "metrics_port", workspace.ManifestPortBinding{})
	require.NoError(t, err)
	assert.Equal(t, 30000, port)
}

func intPtr(v int) *int { return &v }

func TestPortAllocatorFixedPortOutsidePoolAllowed(t *testing.T) {
	alloc := newPortAllocator(nil, bucket.PortRange{Min: 30000, Max: 30005})

	port, err := alloc.assignFixed("database", "database_port", 5432)
	require.NoError(t, err)
	assert.Equal(t, 5432, port)

	port, err = alloc.assignProvisioned("api", "http_port")
	require.NoError(t, err)
	assert.Equal(t, 30000, port)
}

func TestPortAllocatorProvisionedReuseOutsideNarrowedPool(t *testing.T) {
	existing := data.JobPortAssignments{
		"api": {"http_port": 30001},
	}
	alloc := newPortAllocator(existing, bucket.PortRange{Min: 30000, Max: 30000})

	port, err := alloc.assignProvisioned("api", "http_port")
	require.NoError(t, err)
	assert.Equal(t, 30001, port)
}
