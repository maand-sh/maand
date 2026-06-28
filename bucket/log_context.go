// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"os/exec"
	"strings"

	"github.com/google/uuid"
)

const maxCommandSummaryLen = 200

// RunContext identifies one maand CLI invocation in logs.
type RunContext struct {
	RunID     string
	MaandCmd  string
	UpdateSeq int
}

// CommandContext describes one subprocess executed during a run.
type CommandContext struct {
	Job    string
	Phase  string
	Action string
	Cmd    string
}

// NewRunContext returns a fresh run context for maandCmd.
func NewRunContext(maandCmd string, updateSeq int) RunContext {
	return RunContext{
		RunID:     newRunID(),
		MaandCmd:  maandCmd,
		UpdateSeq: updateSeq,
	}
}

func newRunID() string {
	return uuid.NewString()
}

// SummarizeCommands returns a one-line summary of shell commands.
func SummarizeCommands(commands []string) string {
	if len(commands) == 0 {
		return ""
	}
	if len(commands) == 1 {
		return truncateCommandSummary(commands[0])
	}
	return truncateCommandSummary(strings.Join(commands, " && "))
}

// SummarizeExecCmd returns a one-line summary of an exec.Cmd.
func SummarizeExecCmd(cmd *exec.Cmd) string {
	if cmd == nil {
		return ""
	}
	parts := make([]string, 0, len(cmd.Args)+1)
	if cmd.Path != "" {
		parts = append(parts, cmd.Path)
	}
	if len(cmd.Args) > 0 {
		parts = append(parts, cmd.Args...)
	}
	summary := strings.Join(parts, " ")
	return truncateCommandSummary(summary)
}

func truncateCommandSummary(summary string) string {
	summary = strings.Join(strings.Fields(summary), " ")
	if len(summary) <= maxCommandSummaryLen {
		return summary
	}
	return summary[:maxCommandSummaryLen-3] + "..."
}

func (c CommandContext) withDefaults(workerIP string, cmd *exec.Cmd, commands []string) CommandContext {
	out := c
	if out.Cmd == "" {
		if cmd != nil {
			out.Cmd = SummarizeExecCmd(cmd)
		} else {
			out.Cmd = SummarizeCommands(commands)
		}
	}
	if out.Action == "" && cmd != nil && len(cmd.Args) > 0 {
		out.Action = cmd.Args[0]
	}
	_ = workerIP
	return out
}
