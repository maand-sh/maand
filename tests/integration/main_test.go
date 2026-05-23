// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"maand/bucket"
)

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		os.Exit(1)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(wd, "go.mod")); statErr == nil {
			bucket.Location = filepath.Join(wd, "tests", "integration", "test_bucket")
			break
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			bucket.Location = "./test_bucket"
			break
		}
		wd = parent
	}
	bucket.UpdatePath()
	os.Exit(m.Run())
}
