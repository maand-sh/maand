package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitDefaultBucketConfPortRange(t *testing.T) {
	initFreshBucket(t)

	content, err := os.ReadFile(path.Join(bucket.WorkspaceLocation, "bucket.conf"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "port_min")
	assert.Contains(t, string(content), "port_max")

	r, err := bucket.LoadPortRange()
	require.NoError(t, err)
	assert.Equal(t, 30000, r.Min)
	assert.Equal(t, 39999, r.Max)
}
