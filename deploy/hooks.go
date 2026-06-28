// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"maand/bucket"
	"maand/worker"
)

// TestHooks overrides deploy side effects during tests. Clear with ClearTestHooks when done.
type TestHooks struct {
	WorkerCommand            func(rt *bucket.Runtime, workerIP string, cmdCtx bucket.CommandContext, commands []string, env []string) error
	Rsync                    func(rt *bucket.Runtime, bucketID, workerIP string, jobs []string) error
	SetupRuntime             func(bucketID string, run bucket.RunContext) (*bucket.Runtime, error)
	CheckWorkerPrerequisites func(rt *bucket.Runtime, workers []string) error
}

var testHooks *TestHooks

// SetTestHooks installs deploy test doubles. Not for production use.
func SetTestHooks(h *TestHooks) {
	testHooks = h
}

// ClearTestHooks removes deploy test doubles.
func ClearTestHooks() {
	testHooks = nil
}

// CommandRecorder captures worker commands when used as TestHooks.WorkerCommand.
type CommandRecorder struct {
	BucketID string
	Commands []RecordedCommand
}

// RecordedCommand is one worker command invocation.
type RecordedCommand struct {
	WorkerIP string
	Command  string
}

// Record implements TestHooks.WorkerCommand.
func (r *CommandRecorder) Record(_ *bucket.Runtime, workerIP string, _ bucket.CommandContext, commands []string, _ []string) error {
	for _, cmd := range commands {
		r.Commands = append(r.Commands, RecordedCommand{WorkerIP: workerIP, Command: cmd})
	}
	return nil
}

// HasAction reports whether a runner action was recorded for a job on a worker.
func (r *CommandRecorder) HasAction(workerIP, action, job string) bool {
	want := runnerCommand(r.BucketID, action, job)
	for _, c := range r.Commands {
		if c.WorkerIP == workerIP && c.Command == want {
			return true
		}
	}
	return false
}

func runWorkerCommand(rt *bucket.Runtime, workerIP string, cmdCtx bucket.CommandContext, commands []string, env []string) error {
	if testHooks != nil && testHooks.WorkerCommand != nil {
		return testHooks.WorkerCommand(rt, workerIP, cmdCtx, commands, env)
	}
	return worker.ExecuteCommand(rt, workerIP, cmdCtx, commands, env)
}

func runRsync(rt *bucket.Runtime, bucketID, workerIP string, jobs []string) error {
	if testHooks != nil && testHooks.Rsync != nil {
		return testHooks.Rsync(rt, bucketID, workerIP, jobs)
	}
	return rsync(rt, bucketID, workerIP, jobs)
}

func setupDeployRuntime(bucketID string, run bucket.RunContext) (*bucket.Runtime, error) {
	if testHooks != nil && testHooks.SetupRuntime != nil {
		return testHooks.SetupRuntime(bucketID, run)
	}
	return bucket.SetupRuntime(bucketID, run)
}

func runnerCmdCtx(job, phase, action, bucketID string) bucket.CommandContext {
	cmd := runnerCommand(bucketID, action, job)
	return bucket.CommandContext{
		Job:    job,
		Phase:  phase,
		Action: action,
		Cmd:    cmd,
	}
}
