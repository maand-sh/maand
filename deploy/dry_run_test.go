// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"bytes"
	"io"
	"os"
	"path"
	"testing"

	"maand/kv"

	"github.com/stretchr/testify/require"
)

func TestDryRun_noDeploymentAfterPromote(t *testing.T) {
	env := setupDeployTestEnv(t)
	installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil))

	result, err := DryRun(nil)
	require.NoError(t, err)
	require.False(t, result.Required)
	require.Len(t, result.Jobs, 1)
	require.Equal(t, "app", result.Jobs[0].Job)
	require.False(t, result.Jobs[0].NeedsRollout)
}

func TestDryRun_detectsContentChange(t *testing.T) {
	env := setupDeployTestEnv(t)
	installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil))

	tx = env.begin(t)
	env.insertJobFile(t, tx, "job-app", path.Join("app", "marker.txt"), "v2", false)
	require.NoError(t, tx.Commit())

	result, err := DryRun(nil)
	require.NoError(t, err)
	require.True(t, result.Required)
	require.Len(t, result.Jobs, 1)
	require.True(t, result.Jobs[0].NeedsRollout)
	require.Len(t, result.Jobs[0].Allocations, 1)
	require.Equal(t, rolloutActionRestart, result.Jobs[0].Allocations[0].Action)
	require.NotEmpty(t, result.Jobs[0].Allocations[0].PreviousHash)
	require.NotEmpty(t, result.Jobs[0].Allocations[0].CurrentHash)
	require.NotEqual(
		t,
		result.Jobs[0].Allocations[0].PreviousHash,
		result.Jobs[0].Allocations[0].CurrentHash,
	)
}

func TestDryRun_firstDeploy(t *testing.T) {
	env := setupDeployTestEnv(t)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	result, err := DryRun(nil)
	require.NoError(t, err)
	require.True(t, result.Required)
	require.Equal(t, rolloutActionStart, result.Jobs[0].Allocations[0].Action)
}

func TestDryRun_doesNotPersistHashes(t *testing.T) {
	env := setupDeployTestEnv(t)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	result, err := DryRun(nil)
	require.NoError(t, err)
	require.True(t, result.Required)

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	needs, err := JobNeedsRollout(tx, "app")
	require.NoError(t, err)
	require.True(t, needs, "dry-run must not write hashes that skip a real deploy")
	require.NoError(t, tx.Rollback())
}

func TestPrintDryRun(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	PrintDryRun(DryRunResult{
		Required: true,
		Jobs: []JobPlan{{
			Job:           "app",
			DeploymentSeq: 0,
			NeedsRollout:  true,
			Allocations: []AllocationPlan{{
				WorkerIP:     "10.0.0.1",
				Action:       rolloutActionRestart,
				PreviousHash: "aaa",
				CurrentHash:  "bbb",
			}},
		}},
	})

	require.NoError(t, w.Close())
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "deployment required")
	require.Contains(t, out, `job "app"`)
	require.Contains(t, out, "restart")
	require.Contains(t, out, "previous_hash=aaa")
	require.Contains(t, out, "current_hash=bbb")
}
