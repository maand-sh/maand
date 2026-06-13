package tests

import (
	"testing"

	"maand/build"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildKVJobAndAllocationVariables(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[
		{"host":"10.0.0.1","labels":["svc"],"position":1,"hostname":"w1"},
		{"host":"10.0.0.2","labels":["svc"],"position":0,"hostname":"w2"}
	]`)
	writeMinimalJob(t, "svc", `{
		"selectors": ["svc"],
		"resources": {"ports": {"svc_http_port": {}, "svc_cql_port": {}}}
	}`)

	require.NoError(t, build.Execute())

	workers, _ := GetKey("maand/job/svc", "workers")
	assert.Equal(t, "10.0.0.2,10.0.0.1", workers)

	length, _ := GetKey("maand/job/svc", "workers_length")
	assert.Equal(t, "2", length)

	worker0, _ := GetKey("maand/job/svc", "worker_0")
	worker1, _ := GetKey("maand/job/svc", "worker_1")
	assert.Equal(t, "10.0.0.2", worker0)
	assert.Equal(t, "10.0.0.1", worker1)

	httpPort, _ := GetKey("maand", "svc_http_port")
	cqlPort, _ := GetKey("maand", "svc_cql_port")
	assert.NotEmpty(t, httpPort)
	assert.NotEmpty(t, cqlPort)

	_, err := GetKey("maand/job/svc", "ports_json")
	assert.Error(t, err)

	jobs, _ := GetKey("maand", "jobs")
	assert.Equal(t, "svc", jobs)
	bucketID, _ := GetKey("maand", "bucket_id")
	assert.NotEmpty(t, bucketID)

	position, _ := GetKey("maand/worker/10.0.0.1", "position")
	assert.Equal(t, "1", position)
	hostname, _ := GetKey("maand/worker/10.0.0.1", "hostname")
	assert.Equal(t, "w1", hostname)

	idx, _ := GetKey("maand/job/svc/worker/10.0.0.2", "svc_allocation_index")
	assert.Equal(t, "0", idx)
	peers, _ := GetKey("maand/job/svc/worker/10.0.0.2", "peer_workers")
	assert.Equal(t, "10.0.0.1", peers)
}
