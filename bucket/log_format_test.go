// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatKVLine(t *testing.T) {
	line := formatKVLine(map[string]string{
		"ts":     "2026-06-02T12:00:00.000Z",
		"level":  "INFO",
		"event":  "command_begin",
		"run":    "abc",
		"maand":  "deploy",
		"worker": "10.0.0.1",
		"job":    "api",
		"cmd":    `python3 runner.py stop api`,
	})
	assert.Contains(t, line, `event=command_begin`)
	assert.Contains(t, line, `worker=10.0.0.1`)
	assert.Contains(t, line, `cmd="python3 runner.py stop api"`)
}

func TestFormatJSONLine(t *testing.T) {
	line := formatJSONLine(map[string]string{
		"ts":    "2026-06-02T12:00:00.000Z",
		"event": "deploy_skip",
		"job":   "api",
		"seq":   "3",
	})
	assert.True(t, strings.HasPrefix(line, "{"))
	assert.Contains(t, line, `"event":"deploy_skip"`)
	assert.Contains(t, line, `"seq":3`)
}

func TestSummarizeCommandsTruncates(t *testing.T) {
	long := strings.Repeat("x", maxCommandSummaryLen+10)
	got := SummarizeCommands([]string{long})
	require.LessOrEqual(t, len(got), maxCommandSummaryLen)
	assert.True(t, strings.HasSuffix(got, "..."))
}

func TestFormatCommandBeginEnd(t *testing.T) {
	run := NewRunContext("deploy", 7)
	cmdCtx := CommandContext{Job: "api", Phase: "rollout", Action: "start", Cmd: "make start"}
	begin := formatCommandBegin(run, "10.0.0.1", cmdCtx)
	end := formatCommandEnd(run, "10.0.0.1", cmdCtx, 0, 1500*time.Millisecond)
	assert.Contains(t, begin, "event=command_begin")
	assert.Contains(t, end, "event=command_end")
	assert.Contains(t, end, "duration_ms=1500")
}

func TestParseLogFormat(t *testing.T) {
	assert.Equal(t, LogFormatJSON, parseLogFormat("json"))
	assert.Equal(t, LogFormatKV, parseLogFormat(""))
}

func TestFormatStreamLine(t *testing.T) {
	run := NewRunContext("deploy", 14)
	cmdCtx := CommandContext{Job: "cassandra", Phase: "rsync", Action: "rsync"}
	line := formatStreamLine(run, "10.0.0.1", cmdCtx, "stdout", "Transfer starting: 18 files")
	assert.Contains(t, line, "worker=10.0.0.1")
	assert.Contains(t, line, "job=cassandra")
	assert.Contains(t, line, "phase=rsync")
	assert.Contains(t, line, "stream=stdout")
	assert.Contains(t, line, `msg="Transfer starting: 18 files"`)
	assert.Contains(t, line, "run="+run.RunID)
	assert.Contains(t, line, "seq=14")
}

func TestFormatCLIStreamLine(t *testing.T) {
	cmdCtx := CommandContext{Job: "cassandra", Phase: "job_command", Action: "command_cluster_status"}
	assert.Equal(t, "cassandra@10.48.200.1 | seed: 10.48.200.1", formatCLIStreamLine("10.48.200.1", cmdCtx, "stdout", "seed: 10.48.200.1"))
	assert.Equal(t, "cassandra@10.48.200.1 ! error", formatCLIStreamLine("10.48.200.1", cmdCtx, "stderr", "error"))
	assert.Equal(t, "cassandra | summary", formatCLIStreamLine("-", cmdCtx, "stdout", "summary"))
	assert.Equal(t, "10.48.200.1 | line", formatCLIStreamLine("10.48.200.1", CommandContext{}, "stdout", "line"))
}
