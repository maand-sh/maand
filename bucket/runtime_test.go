// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunCommandStructuredLogging(t *testing.T) {
	root := t.TempDir()
	origLocation := Location
	origLogLocation := LogLocation
	origRunLogLocation := RunLogLocation
	Location = root
	UpdatePath()
	t.Cleanup(func() {
		Location = origLocation
		UpdatePath()
		LogLocation = origLogLocation
		RunLogLocation = origRunLogLocation
	})

	run := NewRunContext("test", 1)
	rt, err := SetupRuntime("bucket-1", run)
	require.NoError(t, err)

	cmd := exec.Command("bash", "-c", "echo hello")
	cmdCtx := CommandContext{
		Job:    "api",
		Phase:  "rollout",
		Action: "start",
		Cmd:    "echo hello",
	}
	require.NoError(t, rt.RunCommand("10.0.0.1", cmdCtx, cmd))

	workerLog := filepath.Join(LogLocation, "10.0.0.1.log")
	data, err := os.ReadFile(workerLog)
	require.NoError(t, err)
	text := string(data)
	require.Contains(t, text, "event=command_begin")
	require.Contains(t, text, "event=command_end")
	require.Contains(t, text, "worker=10.0.0.1")
	require.Contains(t, text, "stream=stdout")
	require.Contains(t, text, "msg=hello")
	require.Contains(t, text, `run=`+run.RunID)

	runLog := filepath.Join(RunLogLocation, run.RunID, "10.0.0.1.log")
	runData, err := os.ReadFile(runLog)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(runData), "command_begin"))
}

func TestLogEvent(t *testing.T) {
	root := t.TempDir()
	origLocation := Location
	Location = root
	UpdatePath()
	t.Cleanup(func() {
		Location = origLocation
		UpdatePath()
	})

	run := NewRunContext("deploy", 2)
	rt, err := SetupRuntime("bucket-1", run)
	require.NoError(t, err)

	require.NoError(t, rt.LogEvent("", "deploy_skip", map[string]string{
		"job":    "api",
		"reason": "already_promoted",
	}))

	data, err := os.ReadFile(filepath.Join(LogLocation, "maand.log"))
	require.NoError(t, err)
	require.Contains(t, string(data), "event=deploy_skip")
	require.Contains(t, string(data), "job=api")
}
