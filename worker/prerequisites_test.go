package worker

import (
	"testing"

	"maand/prereq"

	"github.com/stretchr/testify/assert"
)

func TestDeployWorkerSpecScript(t *testing.T) {
	script := prereq.BuildWorkerCheckScript(prereq.DeployWorkerSpec)
	assert.Contains(t, script, "check python3")
}
