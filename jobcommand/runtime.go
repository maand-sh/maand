// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"fmt"
	"os"
	"path"

	"maand/bucket"
)

// Runtime identifies how a job command script is executed on the host.
type Runtime string

const (
	RuntimePython Runtime = "python"
	RuntimeBun    Runtime = "bun"
)

const (
	commandScriptPython = ".py"
	commandScriptBunTS  = ".ts"
	commandScriptBunJS  = ".js"
)

// ResolveCommandScript picks the command implementation file under modulesDir.
// Exactly one of command_<name>.py, .ts, or .js must exist.
func ResolveCommandScript(modulesDir, commandName string) (Runtime, string, error) {
	candidates := []struct {
		suffix  string
		runtime Runtime
	}{
		{commandScriptPython, RuntimePython},
		{commandScriptBunTS, RuntimeBun},
		{commandScriptBunJS, RuntimeBun},
	}

	var (
		foundRuntime Runtime
		foundPath    string
		foundCount   int
	)

	for _, candidate := range candidates {
		scriptPath := path.Join(modulesDir, commandName+candidate.suffix)
		if _, err := os.Stat(scriptPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", "", err
		}
		foundRuntime = candidate.runtime
		foundPath = scriptPath
		foundCount++
	}

	switch foundCount {
	case 0:
		return "", "", fmt.Errorf(
			"%w: expected %s.py, %s.ts, or %s.js under %s",
			bucket.ErrJobCommandFileNotFound,
			commandName,
			commandName,
			commandName,
			modulesDir,
		)
	case 1:
		return foundRuntime, foundPath, nil
	default:
		return "", "", fmt.Errorf(
			"%w: multiple implementations for %s under %s (use only one of .py, .ts, .js)",
			bucket.ErrInvalidJobCommandConfiguration,
			commandName,
			modulesDir,
		)
	}
}

// CommandExecLines returns shell commands to run scriptPath with runtime from moduleDir.
// jobName selects a per-job virtualenv under workspace/jobs/<job>/_modules/.venv when present.
func CommandExecLines(moduleDir, scriptPath string, runtime Runtime, jobName string) []string {
	scriptName := path.Base(scriptPath)
	switch runtime {
	case RuntimeBun:
		return []string{
			fmt.Sprintf("cd %s", moduleDir),
			"bun run " + scriptName,
		}
	default:
		python := ResolvePythonExecutable(jobName)
		return []string{
			fmt.Sprintf("cd %s", moduleDir),
			python + " " + scriptName,
		}
	}
}
