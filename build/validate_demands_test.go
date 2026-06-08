// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"maand/bucket"
	"maand/workspace"
)

func setupDemandValidationWorkspace(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})
	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
}

func writeDemandValidationJob(t *testing.T, job, manifest string) {
	t.Helper()
	jobPath := path.Join(bucket.WorkspaceLocation, "jobs", job)
	require.NoError(t, os.MkdirAll(jobPath, 0o755))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "manifest.json"), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(path.Join(jobPath, "Makefile"), []byte(""), 0o644))
}

func TestValidateJobCommandDemands_noDemands(t *testing.T) {
	setupDemandValidationWorkspace(t)
	writeDemandValidationJob(t, "api", `{"selectors":["web"],"version":"1.0.0"}`)

	err := ValidateJobCommandDemands(workspace.Default(), []string{"api"})
	require.NoError(t, err)
}

func TestValidateJobCommandDemands_validDemand(t *testing.T) {
	setupDemandValidationWorkspace(t)
	writeDemandValidationJob(t, "db", `{
		"selectors":["db"],
		"version":"2.1.0",
		"commands":{"command_schema":{"executed_on":["cli"]}}
	}`)
	writeDemandValidationJob(t, "api", `{
		"selectors":["web"],
		"version":"1.0.0",
		"commands":{"command_migrate":{"executed_on":["cli"],"demands":{"job":"db","command":"command_schema","config":{"min_version":"2.0.0","max_version":"3.0.0"}}}}
	}`)

	err := ValidateJobCommandDemands(workspace.Default(), []string{"api", "db"})
	require.NoError(t, err)
}

func TestValidateJobCommandDemands_unknownJob(t *testing.T) {
	setupDemandValidationWorkspace(t)
	writeDemandValidationJob(t, "api", `{
		"selectors":["web"],
		"version":"1.0.0",
		"commands":{"command_migrate":{"executed_on":["cli"],"demands":{"job":"missing","command":"command_schema"}}}
	}`)

	err := ValidateJobCommandDemands(workspace.Default(), []string{"api"})
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidJobCommandDemand)
}

func TestValidateJobCommandDemands_unknownCommand(t *testing.T) {
	setupDemandValidationWorkspace(t)
	writeDemandValidationJob(t, "db", `{"selectors":["db"],"version":"1.0.0"}`)
	writeDemandValidationJob(t, "api", `{
		"selectors":["web"],
		"version":"1.0.0",
		"commands":{"command_migrate":{"executed_on":["cli"],"demands":{"job":"db","command":"command_schema"}}}
	}`)

	err := ValidateJobCommandDemands(workspace.Default(), []string{"api", "db"})
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidJobCommandDemand)
}

func TestValidateJobCommandDemands_requiresUpstreamVersion(t *testing.T) {
	setupDemandValidationWorkspace(t)
	writeDemandValidationJob(t, "db", `{
		"selectors":["db"],
		"commands":{"command_schema":{"executed_on":["cli"]}}
	}`)
	writeDemandValidationJob(t, "api", `{
		"selectors":["web"],
		"version":"1.0.0",
		"commands":{"command_migrate":{"executed_on":["cli"],"demands":{"job":"db","command":"command_schema","config":{"min_version":"1.0.0"}}}}
	}`)

	err := ValidateJobCommandDemands(workspace.Default(), []string{"api", "db"})
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidJobVersion)
}

func TestValidateJobCommandDemands_versionMismatch(t *testing.T) {
	setupDemandValidationWorkspace(t)
	writeDemandValidationJob(t, "db", `{
		"selectors":["db"],
		"version":"1.0.0",
		"commands":{"command_schema":{"executed_on":["cli"]}}
	}`)
	writeDemandValidationJob(t, "api", `{
		"selectors":["web"],
		"version":"1.0.0",
		"commands":{"command_migrate":{"executed_on":["cli"],"demands":{"job":"db","command":"command_schema","config":{"min_version":"2.0.0"}}}}
	}`)

	err := ValidateJobCommandDemands(workspace.Default(), []string{"api", "db"})
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrJobCommandDemandVersionMismatch)
}

func TestLoadJobCatalog_resolvesVersions(t *testing.T) {
	setupDemandValidationWorkspace(t)
	writeDemandValidationJob(t, "api", `{"selectors":["web"],"version":"1.2.3"}`)

	catalog, err := loadJobCatalog(workspace.Default(), []string{"api"})
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", catalog.versions["api"].String())
}

func TestJobCatalog_isDemandTarget(t *testing.T) {
	setupDemandValidationWorkspace(t)
	writeDemandValidationJob(t, "db", `{"selectors":["db"],"version":"1.0.0","commands":{"command_schema":{"executed_on":["cli"]}}}`)
	writeDemandValidationJob(t, "api", `{
		"selectors":["web"],
		"version":"1.0.0",
		"commands":{"command_migrate":{"executed_on":["cli"],"demands":{"job":"db","command":"command_schema"}}}
	}`)

	catalog, err := loadJobCatalog(workspace.Default(), []string{"api", "db"})
	require.NoError(t, err)
	assert.True(t, catalog.isDemandTarget("db"))
	assert.False(t, catalog.isDemandTarget("api"))
}
