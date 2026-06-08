package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJobDeployArtifactsCleanupCommandPreservesDataAndLogs(t *testing.T) {
	cmd := JobDeployArtifactsCleanupCommand("bucket-1", "vault")
	assert.Contains(t, cmd, `/opt/worker/bucket-1/jobs/vault`)
	assert.Contains(t, cmd, `! -name data`)
	assert.Contains(t, cmd, `! -name logs`)
	assert.NotContains(t, cmd, `rm -rf /opt/worker/bucket-1/jobs/vault`)
}
