// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"os"
	"path/filepath"
)

// ResolveRoot sets Location to the directory containing maand.conf or data/maand.db
// by searching the current directory and its parents.
func ResolveRoot() error {
	if isBucketRoot(Location) {
		abs, err := filepath.Abs(Location)
		if err != nil {
			return UnexpectedError(err)
		}
		Location = abs
		UpdatePath()
		return nil
	}

	start, err := os.Getwd()
	if err != nil {
		return UnexpectedError(err)
	}

	for dir := start; ; dir = filepath.Dir(dir) {
		if isBucketRoot(dir) {
			abs, err := filepath.Abs(dir)
			if err != nil {
				return UnexpectedError(err)
			}
			Location = abs
			UpdatePath()
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return nil
}

func isBucketRoot(dir string) bool {
	if dir == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "maand.conf")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "maand.db")); err == nil {
		return true
	}
	return false
}

// Root returns the absolute bucket root after ResolveRoot.
func Root() (string, error) {
	if err := ResolveRoot(); err != nil {
		return "", err
	}
	return filepath.Abs(Location)
}
