// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"os"
	"path"
	"path/filepath"

	"maand/bucket"
)

// WorkspaceJobModulesDir is workspace/jobs/<job>/_modules (where you create .venv).
func WorkspaceJobModulesDir(jobName string) string {
	return path.Join(bucket.WorkspaceLocation, "jobs", jobName, "_modules")
}

// ResolvePythonExecutable returns the Python interpreter for job commands.
// Prefers workspace/jobs/<job>/_modules/.venv/bin/python3 (or venv/), else python3 on PATH.
func ResolvePythonExecutable(jobName string) string {
	modulesDir := WorkspaceJobModulesDir(jobName)
	for _, venvDir := range []string{".venv", "venv"} {
		for _, name := range []string{"python3", "python"} {
			candidate := filepath.Join(modulesDir, venvDir, "bin", name)
			info, err := os.Stat(candidate)
			if err == nil && !info.IsDir() {
				abs, err := filepath.Abs(candidate)
				if err == nil {
					return abs
				}
				return candidate
			}
		}
	}
	return "python3"
}
