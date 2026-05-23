// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package prereq

import (
	"os"
	"path/filepath"
	"strings"

	"maand/bucket"
)

// WorkspaceUsesBun reports whether any job command module uses Bun (.ts/.js).
func WorkspaceUsesBun() (bool, error) {
	jobsRoot := filepath.Join(bucket.WorkspaceLocation, "jobs")
	entries, err := os.ReadDir(jobsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		modulesDir := filepath.Join(jobsRoot, entry.Name(), "_modules")
		usesBun, err := modulesDirUsesBun(modulesDir)
		if err != nil {
			return false, err
		}
		if usesBun {
			return true, nil
		}
	}
	return false, nil
}

func modulesDirUsesBun(modulesDir string) (bool, error) {
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "command_") {
			continue
		}
		switch filepath.Ext(name) {
		case ".ts", ".js":
			return true, nil
		}
	}
	return false, nil
}
