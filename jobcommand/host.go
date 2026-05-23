// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"maand/prereq"
)

func validateHostRuntime(jobName, commandName string) error {
	moduleDir := WorkspaceJobModulesDir(jobName)
	runtime, _, err := ResolveCommandScript(moduleDir, commandName)
	if err != nil {
		return err
	}

	switch runtime {
	case RuntimeBun:
		return prereq.CheckLocal("bun")
	default:
		return prereq.CheckLocal(ResolvePythonExecutable(jobName))
	}
}
