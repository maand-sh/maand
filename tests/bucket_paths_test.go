// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"os"
	"path/filepath"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setBucketRootForTest(t *testing.T, root string) {
	t.Helper()
	prevLocation := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = prevLocation
		bucket.UpdatePath()
	})
}

func TestBucketPathMapsBucketFiles(t *testing.T) {
	setBucketRootForTest(t, t.TempDir())

	require.NoError(t, os.MkdirAll(filepath.Join(bucket.Location, "tmp"), 0o755))
	scriptPath := filepath.Join(bucket.Location, "tmp", "run.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/bash"), 0o700))

	absPath, err := bucket.BucketPath(scriptPath)
	require.NoError(t, err)
	assert.Equal(t, scriptPath, absPath)
}

func TestBucketPathRejectsPathsOutsideBucket(t *testing.T) {
	root := t.TempDir()
	setBucketRootForTest(t, root)

	outside := filepath.Join(root, "..", "outside.txt")
	_, err := bucket.BucketPath(outside)
	require.Error(t, err)
}
