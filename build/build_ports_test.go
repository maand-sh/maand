package build

import (
	"testing"

	"maand/bucket"
	"maand/data"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortAllocatorStableAndReuse(t *testing.T) {
	existing := data.JobPortAssignments{
		"a": {"web_port": 30001},
	}
	alloc := newPortAllocator(existing, bucket.PortRange{Min: 30000, Max: 30002})

	port, err := alloc.assign("a", "web_port")
	require.NoError(t, err)
	assert.Equal(t, 30001, port)

	port, err = alloc.assign("b", "api_port")
	require.NoError(t, err)
	assert.Equal(t, 30000, port)

	port, err = alloc.assign("b", "api_port")
	require.NoError(t, err)
	assert.Equal(t, 30000, port)
}

func TestPortAllocatorReleaseRemoved(t *testing.T) {
	existing := data.JobPortAssignments{
		"a": {"web_port": 30000, "admin_port": 30001},
	}
	alloc := newPortAllocator(existing, bucket.PortRange{Min: 30000, Max: 30001})

	alloc.releaseRemoved("a", []string{"admin_port"})

	port, err := alloc.assign("b", "other_port")
	require.NoError(t, err)
	assert.Equal(t, 30000, port)
}

func TestPortAllocatorExhausted(t *testing.T) {
	existing := data.JobPortAssignments{
		"a": {"p1": 30000},
	}
	alloc := newPortAllocator(existing, bucket.PortRange{Min: 30000, Max: 30000})

	_, err := alloc.assign("b", "p2")
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrPortRangeExhausted)
}
