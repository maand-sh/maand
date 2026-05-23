package prereq

import (
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckLocal(t *testing.T) {
	require.NoError(t, CheckLocal("bash"))
}

func TestCheckLocalMissing(t *testing.T) {
	err := CheckLocal("maand-definitely-not-a-command-xyz")
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrHostPrerequisites)
}

func TestBuildWorkerCheckScriptDeploy(t *testing.T) {
	script := BuildWorkerCheckScript(DeployWorkerSpec)
	assert.Contains(t, script, "check python3")
	assert.Contains(t, script, "sudo rsync --version")
}

func TestBuildWorkerCheckScriptRunCommand(t *testing.T) {
	script := BuildWorkerCheckScript(RunCommandWorkerSpec)
	assert.Contains(t, script, "check bash")
	assert.NotContains(t, script, "check python3")
	assert.NotContains(t, script, "sudo rsync")
}

func TestParseMissingPrerequisites(t *testing.T) {
	assert.Equal(t, []string{"python3"}, ParseMissingPrerequisites("missing: python3\n"))
}
