// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRootFromParentDirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "data"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "maand.conf"), []byte("ssh_user=\"root\"\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "workspace", "jobs"), 0o755))

	sub := filepath.Join(root, "workspace", "jobs")
	require.NoError(t, os.Chdir(sub))

	orig := Location
	t.Cleanup(func() {
		_ = os.Chdir(root)
		Location = orig
		UpdatePath()
	})

	require.NoError(t, ResolveRoot())
	got, err := filepath.EvalSymlinks(Location)
	require.NoError(t, err)
	want, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestResolveRootAlreadyAtBucket(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "maand.conf"), []byte("ssh_user=\"root\"\n"), 0o644))

	orig := Location
	Location = root
	UpdatePath()
	t.Cleanup(func() {
		Location = orig
		UpdatePath()
	})

	require.NoError(t, ResolveRoot())
	assert.Equal(t, root, Location)
}
