// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"io/fs"
	"os"
	"path"
	"strings"

	"maand/bucket"
)

var skipJobWalkDirNames = map[string]struct{}{
	".venv":        {},
	"venv":         {},
	"node_modules": {},
	"__pycache__":  {},
}

// JobFilePath returns an absolute path under workspace/jobs/.
func JobFilePath(relativePath string) string {
	return path.Join(bucket.WorkspaceLocation, "jobs", relativePath)
}

// GetJobFilePath is deprecated; use JobFilePath.
func GetJobFilePath(fpath string) string {
	return JobFilePath(fpath)
}

// WalkJobFiles walks files for jobName under workspace/jobs/.
// Skips .venv, venv, node_modules, and __pycache__ trees.
func WalkJobFiles(jobName string, callback func(path string, d fs.DirEntry, err error) error) error {
	return fs.WalkDir(os.DirFS(path.Join(bucket.WorkspaceLocation, "jobs")), jobName, func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if shouldSkipJobWalk(rel, d) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		return callback(rel, d, err)
	})
}

func shouldSkipJobWalk(rel string, d fs.DirEntry) bool {
	if d.IsDir() {
		if _, skip := skipJobWalkDirNames[d.Name()]; skip {
			return true
		}
	}
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if _, skip := skipJobWalkDirNames[part]; skip {
			return true
		}
	}
	return false
}

// JobHasMakefileOrTemplate reports whether the job defines Makefile or Makefile.tpl.
func JobHasMakefileOrTemplate(jobName string) bool {
	jobDir := JobFilePath(jobName)
	for _, name := range []string{"Makefile", "Makefile.tpl"} {
		if _, err := os.Stat(path.Join(jobDir, name)); err == nil {
			return true
		}
	}
	return false
}
