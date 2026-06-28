// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"path"
	"testing"

	"maand/data"
	"maand/kv"
	"maand/workspace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEffectiveUpdateAction(t *testing.T) {
	assert.Equal(t, rolloutActionRestart, effectiveUpdateAction(false, workspace.RestartPolicyAlways))
	assert.Equal(t, rolloutActionReload, effectiveUpdateAction(false, workspace.RestartPolicyReload))
	assert.Equal(t, rolloutActionSync, effectiveUpdateAction(false, workspace.RestartPolicyNever))
	assert.Equal(t, rolloutActionSync, effectiveUpdateAction(true, workspace.RestartPolicyAlways))
}

func TestResolveUpdateAction_restartGlobs(t *testing.T) {
	action, matched := resolveUpdateAction(false, workspace.RestartPolicyReload, []string{"Makefile"},
		data.FileManifest{"config/app.toml": "1"},
		data.FileManifest{"config/app.toml": "2"},
		true, false, false,
	)
	assert.Equal(t, rolloutActionReload, action)
	assert.Empty(t, matched)

	action, matched = resolveUpdateAction(false, workspace.RestartPolicyReload, []string{"Makefile"},
		data.FileManifest{"Makefile": "1"},
		data.FileManifest{"Makefile": "2", "config/app.toml": "1"},
		true, false, false,
	)
	assert.Equal(t, rolloutActionRestart, action)
	assert.Equal(t, []string{"Makefile"}, matched)
}

func TestResolveUpdateAction_legacyManifestFallback(t *testing.T) {
	action, matched := resolveUpdateAction(false, workspace.RestartPolicyReload, []string{"Makefile"},
		nil, nil, true, false, true,
	)
	assert.Equal(t, rolloutActionReload, action)
	assert.Empty(t, matched)
}

func TestResolveUpdateAction_versionOnlyReload(t *testing.T) {
	action, matched := resolveUpdateAction(false, workspace.RestartPolicyReload, []string{"Makefile"},
		nil, nil, false, true, false,
	)
	assert.Equal(t, rolloutActionReload, action)
	assert.Empty(t, matched)
}

func TestHandleUpdatedAllocations_reloadPolicy(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "new", "old")
	env.setRestartPolicy(t, tx, "app", workspace.RestartPolicyReload)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, handleUpdatedAllocations(tx, nil, env.bucketID, "app", Options{}))
	assert.True(t, rec.HasAction("10.0.0.1", "reload", "app"))
	assert.False(t, rec.HasAction("10.0.0.1", "restart", "app"))
	require.NoError(t, tx.Rollback())
}

func TestHandleUpdatedAllocations_neverPolicySkipsLifecycle(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "new", "old")
	env.setRestartPolicy(t, tx, "app", workspace.RestartPolicyNever)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, handleUpdatedAllocations(tx, nil, env.bucketID, "app", Options{}))
	assert.Empty(t, rec.Commands)
	require.NoError(t, tx.Rollback())
}

func TestDeployJob_syncOnlyPromotesWithoutLifecycle(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "new", "old")
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, prepareJobsFiles(tx, []string{"app"}))
	require.NoError(t, updateAllocationHash(tx, []string{"app"}))
	require.NoError(t, deployJob(tx, nil, env.bucketID, "app", Options{SyncOnly: true}))
	assert.Empty(t, rec.Commands)
	assert.True(t, env.allocationHashPromoted(t, tx, "app", "alloc-app-10.0.0.1"))
	require.NoError(t, tx.Rollback())
}

func TestDeployJob_syncOnlyErrorsOnNewAllocation(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, prepareJobsFiles(tx, []string{"app"}))
	require.NoError(t, updateAllocationHash(tx, []string{"app"}))

	err := deployJob(tx, nil, env.bucketID, "app", Options{SyncOnly: true})
	require.Error(t, err)
	var jobErr *JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Contains(t, jobErr.Err.Error(), "sync-only deploy cannot start new allocations")
	assert.Empty(t, rec.Commands)
	require.NoError(t, tx.Rollback())
}

func TestDryRun_reloadPolicy(t *testing.T) {
	env := setupDeployTestEnv(t)
	installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setRestartPolicy(t, tx, "app", workspace.RestartPolicyReload)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, Options{}))

	tx = env.begin(t)
	env.insertJobFile(t, tx, "job-app", path.Join("app", "marker.txt"), "v2", false)
	require.NoError(t, tx.Commit())

	result, err := DryRun(nil, Options{})
	require.NoError(t, err)
	require.True(t, result.Required)
	require.Equal(t, rolloutActionReload, result.Jobs[0].Allocations[0].Action)
}

func TestDryRun_syncOnlyErrorsOnNewAllocation(t *testing.T) {
	env := setupDeployTestEnv(t)
	installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	_, err := DryRun(nil, Options{SyncOnly: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sync-only deploy cannot start new allocations")
}

func TestValidateSyncOnlyRollout(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, validateSyncOnlyRollout(tx, "app"))
	require.NoError(t, tx.Rollback())
}

func TestHandleUpdatedAllocations_restartGlobs(t *testing.T) {
	env := setupDeployTestEnv(t)
	rec := installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "new", "old")
	env.setRestartPolicy(t, tx, "app", workspace.RestartPolicyReload)
	env.setRestartGlobs(t, tx, "app", []string{"Makefile"})
	prevEncoded, err := data.FileManifest{"Makefile": "1"}.Encode()
	require.NoError(t, err)
	curEncoded, err := data.FileManifest{"Makefile": "2", "marker.txt": "x"}.Encode()
	require.NoError(t, err)
	_, err = tx.Exec(
		`UPDATE hash SET previous_files = ?, current_files = ? WHERE namespace = 'app_allocation' AND key = 'alloc-app-10.0.0.1'`,
		prevEncoded, curEncoded,
	)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, handleUpdatedAllocations(tx, nil, env.bucketID, "app", Options{}))
	assert.True(t, rec.HasAction("10.0.0.1", "restart", "app"))
	assert.False(t, rec.HasAction("10.0.0.1", "reload", "app"))
	require.NoError(t, tx.Rollback())
}

func TestDryRun_restartGlobsMatchedPaths(t *testing.T) {
	env := setupDeployTestEnv(t)
	installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setRestartPolicy(t, tx, "app", workspace.RestartPolicyReload)
	env.setRestartGlobs(t, tx, "app", []string{"Makefile"})
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, Options{}))

	tx = env.begin(t)
	prevEncoded, err := data.FileManifest{"Makefile": "stale-hash"}.Encode()
	require.NoError(t, err)
	_, err = tx.Exec(
		`UPDATE hash SET previous_files = ? WHERE namespace = 'app_allocation' AND key = 'alloc-app-10.0.0.1'`,
		prevEncoded,
	)
	require.NoError(t, err)
	env.insertJobFile(t, tx, "job-app", path.Join("app", "marker.txt"), "v2", false)
	require.NoError(t, tx.Commit())

	result, err := DryRun(nil, Options{})
	require.NoError(t, err)
	require.True(t, result.Required)
	alloc := result.Jobs[0].Allocations[0]
	require.Equal(t, rolloutActionRestart, alloc.Action)
	assert.Contains(t, alloc.MatchedPaths, "Makefile")
}
