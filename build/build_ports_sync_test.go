package build

import (
	"testing"

	"maand/bucket"
	"maand/workspace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncJobPortsAssignsFromManifest(t *testing.T) {
	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	allocator := newPortAllocator(nil, bucket.PortRange{Min: 30000, Max: 30010})

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, syncJobPorts(tx, "job-api", "api", workspace.ManifestPorts{
		"http_port": {},
	}, allocator))
	require.NoError(t, tx.Commit())

	var port int
	require.NoError(t, db.QueryRow(`SELECT port FROM job_ports WHERE job_id = 'job-api' AND name = 'http_port'`).Scan(&port))
	assert.Equal(t, 30000, port)
}
