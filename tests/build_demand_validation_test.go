package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDemandUnknownJob(t *testing.T) {
	require.NoError(t, os.RemoveAll(bucket.Location))
	require.NoError(t, initialize.Execute())

	writeDemandJob(t, "b", "missing", "command_x", "1.0.0", "")

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrInvalidJobCommandDemand)
}

func TestValidateDemandUnknownCommand(t *testing.T) {
	require.NoError(t, os.RemoveAll(bucket.Location))
	require.NoError(t, initialize.Execute())

	writeBaseJob(t, "a", "command_y", "1.0.0")
	writeDemandJob(t, "b", "a", "command_x", "1.0.0", "")

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrInvalidJobCommandDemand)
}

func TestValidateDemandVersionTooLow(t *testing.T) {
	require.NoError(t, os.RemoveAll(bucket.Location))
	require.NoError(t, initialize.Execute())

	writeBaseJob(t, "a", "command_x", "1.0.0")
	writeDemandJob(t, "b", "a", "command_x", "2.0.0", `,"config":{"min_version":"2.0.0"}`)

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrJobCommandDemandVersionMismatch)
}

func TestValidateDemandVersionOk(t *testing.T) {
	require.NoError(t, os.RemoveAll(bucket.Location))
	require.NoError(t, initialize.Execute())

	writeBaseJob(t, "a", "command_x", "2.1.0")
	writeDemandJob(t, "b", "a", "command_x", "1.0.0", `,"config":{"min_version":"2.0.0","max_version":"3.0.0"}`)

	executeBuild(t)
}

func TestValidateDemandRequiresUpstreamVersion(t *testing.T) {
	require.NoError(t, os.RemoveAll(bucket.Location))
	require.NoError(t, initialize.Execute())

	writeBaseJob(t, "a", "command_x", "")
	writeDemandJob(t, "b", "a", "command_x", "1.0.0", `,"config":{"min_version":"1.0.0"}`)

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrInvalidJobVersion)
}

func writeBaseJob(t *testing.T, job, command, version string) {
	t.Helper()
	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", job)
	require.NoError(t, os.MkdirAll(path.Join(jobPath, "_modules"), 0o755))
	versionField := ""
	if version != "" {
		versionField = `"version":"` + version + `",`
	}
	manifest := `{"selectors":["worker"],` + versionField + `"commands":{"` + command + `":{"executed_on":["cli"]}}}`
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "_modules", command+".py"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))
}

func writeDemandJob(t *testing.T, job, demandJob, demandCommand, version, extra string) {
	t.Helper()
	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", job)
	require.NoError(t, os.MkdirAll(path.Join(jobPath, "_modules"), 0o755))
	command := "command_dep"
	versionField := ""
	if version != "" {
		versionField = `"version":"` + version + `",`
	}
	manifest := `{"selectors":["worker"],` + versionField + `"commands":{"` + command + `":{"executed_on":["cli"],"demands":{"job":"` + demandJob + `","command":"` + demandCommand + `"` + extra + `}}}}`
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "_modules", command+".py"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))
}
