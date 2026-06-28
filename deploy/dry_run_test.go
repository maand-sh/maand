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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDryRun_forceRequiresDeploymentAfterPromote(t *testing.T) {
	env := setupDeployTestEnv(t)
	installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, Options{}))

	result, err := DryRun(nil, Options{Force: true})
	require.NoError(t, err)
	require.True(t, result.Required)
	require.Len(t, result.Jobs, 1)
	require.True(t, result.Jobs[0].NeedsRollout)
	require.Equal(t, rolloutActionRestart, result.Jobs[0].Allocations[0].Action)
}

func TestDryRun_noDeploymentAfterPromote(t *testing.T) {
	env := setupDeployTestEnv(t)
	installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	require.NoError(t, Execute(nil, Options{}))

	result, err := DryRun(nil, Options{})
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

	require.NoError(t, Execute(nil, Options{}))

	tx = env.begin(t)
	env.insertJobFile(t, tx, "job-app", path.Join("app", "marker.txt"), "v2", false)
	require.NoError(t, tx.Commit())

	result, err := DryRun(nil, Options{})
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

	result, err := DryRun(nil, Options{})
	require.NoError(t, err)
	require.True(t, result.Required)
	require.Equal(t, rolloutActionStart, result.Jobs[0].Allocations[0].Action)
}

func TestDryRun_doesNotPersistHashes(t *testing.T) {
	env := setupDeployTestEnv(t)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	result, err := DryRun(nil, Options{})
	require.NoError(t, err)
	require.True(t, result.Required)

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	needs, err := JobNeedsRollout(tx, "app")
	require.NoError(t, err)
	require.True(t, needs, "dry-run must not write hashes that skip a real deploy")
	require.NoError(t, tx.Rollback())
}

func TestPlanJobRollout_startNewAllocation(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	plan, err := planJobRollout(tx, "app", 0, Options{})
	require.NoError(t, err)
	require.True(t, plan.NeedsRollout)
	require.Len(t, plan.Allocations, 1)
	assert.Equal(t, rolloutActionStart, plan.Allocations[0].Action)
	require.NoError(t, tx.Rollback())
}

func TestPlanJobRollout_noAllocations(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.insertJob(t, tx, "orphan", 0, 1)
	require.NoError(t, tx.Rollback())

	tx = env.begin(t)
	plan, err := planJobRollout(tx, "orphan", 0, Options{})
	require.NoError(t, err)
	assert.Equal(t, "no allocations", plan.SkipReason)
	assert.False(t, plan.NeedsRollout)
	require.NoError(t, tx.Rollback())
}

func TestPlanJobRollout_disabledPromotedRequiresStop(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "same", "same")
	_, err := tx.Exec(`UPDATE allocations SET disabled = 1 WHERE job = 'app'`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	plan, err := planJobRollout(tx, "app", 0, Options{})
	require.NoError(t, err)
	require.True(t, plan.NeedsRollout)
	require.Len(t, plan.Allocations, 1)
	assert.Equal(t, rolloutActionStop, plan.Allocations[0].Action)
	require.NoError(t, tx.Rollback())
}

func TestDryRun_disabledPromotedRequiresDeploy(t *testing.T) {
	env := setupDeployTestEnv(t)
	installNoopDeployHooks(t, env.bucketID)

	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, tx.Commit())
	require.NoError(t, Execute(nil, Options{}))

	tx = env.begin(t)
	_, err := tx.Exec(`UPDATE allocations SET disabled = 1 WHERE job = 'app'`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	result, err := DryRun(nil, Options{})
	require.NoError(t, err)
	require.True(t, result.Required)
	require.Len(t, result.Jobs, 1)
	require.True(t, result.Jobs[0].NeedsRollout)
	require.Equal(t, rolloutActionStop, result.Jobs[0].Allocations[0].Action)
}

func TestDryRun_disabledVersionPendingRequiresPromote(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "same", "same")
	_, err := tx.Exec(`UPDATE hash SET current_version = '1.0.0' WHERE namespace = 'app_allocation' AND key = 'alloc-app-10.0.0.1'`)
	require.NoError(t, err)
	_, err = tx.Exec(`UPDATE allocations SET disabled = 1, new_version = '2.0.0' WHERE job = 'app'`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	result, err := DryRun(nil, Options{})
	require.NoError(t, err)
	require.True(t, result.Required)
	require.Equal(t, rolloutActionStopPromote, result.Jobs[0].Allocations[0].Action)
}

func TestPlanJobRollout_forceRestart(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "same", "same")
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	plan, err := planJobRollout(tx, "app", 0, Options{Force: true})
	require.NoError(t, err)
	require.True(t, plan.NeedsRollout)
	require.Len(t, plan.Allocations, 1)
	assert.Equal(t, rolloutActionRestart, plan.Allocations[0].Action)
	require.NoError(t, tx.Rollback())
}

func TestPlanJobRollout_alreadyPromoted(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.setAllocationHash(t, tx, "app", "alloc-app-10.0.0.1", "same", "same")
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	plan, err := planJobRollout(tx, "app", 0, Options{})
	require.NoError(t, err)
	assert.False(t, plan.NeedsRollout)
	assert.Equal(t, "already promoted on all allocations", plan.SkipReason)
	require.Equal(t, rolloutActionSkip, plan.Allocations[0].Action)
	require.NoError(t, tx.Rollback())
}

func TestPrintAllocationPlan_actions(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	printAllocationPlan(AllocationPlan{
		WorkerIP:     "10.0.0.1",
		Action:       rolloutActionRestart,
		PreviousHash: "aaa",
		CurrentHash:  "bbb",
	})

	require.NoError(t, w.Close())
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "10.0.0.1")
	require.Contains(t, out, "restart")
	require.Contains(t, out, "previous_hash=aaa")
	require.Contains(t, out, "current_hash=bbb")
}

func TestPrintDryRun_noJobs(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	PrintDryRun(DryRunResult{})
	require.NoError(t, w.Close())
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "no jobs matched")
}

func TestPrintDryRun_skipJob(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	PrintDryRun(DryRunResult{
		Jobs: []JobPlan{{
			Job:           "app",
			DeploymentSeq: 0,
			NeedsRollout:  false,
			SkipReason:    "already promoted on all allocations",
		}},
	})
	require.NoError(t, w.Close())
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "no deployment required")
	require.Contains(t, buf.String(), "skip")
}

func TestPrintDryRun_skipAndDefaultActions(t *testing.T) {
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
			Allocations: []AllocationPlan{
				{WorkerIP: "10.0.0.1", Action: rolloutActionSkip, CurrentHash: "same"},
				{WorkerIP: "10.0.0.2", Action: "unknown"},
			},
		}},
	})
	require.NoError(t, w.Close())
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "skip")
	require.Contains(t, out, "hash=same")
	require.Contains(t, out, "10.0.0.2  unknown")
}

func TestPrintDryRun_startAction(t *testing.T) {
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
				WorkerIP:    "10.0.0.1",
				Action:      rolloutActionStart,
				CurrentHash: "abc",
			}},
		}},
	})
	require.NoError(t, w.Close())
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "start")
	require.Contains(t, buf.String(), "current_hash=abc")
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
