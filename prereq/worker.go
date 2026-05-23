// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package prereq

import (
	"strings"
)

// WorkerSpec selects remote commands to verify over SSH.
type WorkerSpec struct {
	Commands  []string
	UseSudo   bool
	SudoRsync bool
}

// DeployWorkerSpec is the prerequisite set for deploy/rsync/runner on workers.
var DeployWorkerSpec = WorkerSpec{
	Commands:  []string{"python3", "make", "rsync", "bash", "timeout"},
	UseSudo:   true,
	SudoRsync: true,
}

// RunCommandWorkerSpec is the prerequisite set for run_command on workers.
var RunCommandWorkerSpec = WorkerSpec{
	Commands:  []string{"bash", "timeout"},
	UseSudo:   true,
	SudoRsync: false,
}

// BuildWorkerCheckScript returns a bash script that exits 1 when tools are missing.
func BuildWorkerCheckScript(spec WorkerSpec) string {
	lines := []string{
		"#!/bin/bash",
		"set -u",
		"missing=()",
		`check() { command -v "$1" >/dev/null 2>&1 || missing+=("$1"); }`,
	}
	for _, command := range spec.Commands {
		lines = append(lines, "check "+command)
	}
	if spec.UseSudo {
		lines = append(lines, "check sudo")
	}
	if spec.SudoRsync {
		lines = append(lines,
			`if command -v sudo >/dev/null 2>&1 && command -v rsync >/dev/null 2>&1; then`,
			`  sudo rsync --version >/dev/null 2>&1 || missing+=("sudo-rsync")`,
			"fi",
		)
	}
	lines = append(lines,
		`if [ ${#missing[@]} -gt 0 ]; then`,
		`  echo "missing: ${missing[*]}"`,
		"  exit 1",
		"fi",
	)
	return strings.Join(lines, "\n") + "\n"
}

func ParseMissingPrerequisites(output string) []string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "missing:") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, "missing:"))
		if raw == "" {
			return nil
		}
		return strings.Fields(raw)
	}
	return nil
}
