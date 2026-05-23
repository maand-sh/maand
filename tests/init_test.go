// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const workerJSONFixture = `[{"host": "10.0.0.1"}, {"host":"10.0.0.2"}]`

func TestInitCreatesDatabaseAndSchemaVersion(t *testing.T) {
	initFreshBucket(t)

	require.True(t, data.DatabaseExists())
	requireLatestSchema(t)
	assert.True(t, mustTableExists(t, "bucket"))
	assert.True(t, mustTableExists(t, "schema_version"))
}

func TestInitUpgradePreservesBucketID(t *testing.T) {
	initFreshBucket(t)
	originalBucketID := mustGetBucketID(t)

	upgradeBucket(t)

	assert.Equal(t, originalBucketID, mustGetBucketID(t))
	requireLatestSchema(t)
}

func TestInitUpgradeIsIdempotent(t *testing.T) {
	initFreshBucket(t)

	for i := 0; i < 3; i++ {
		require.NoError(t, initialize.Execute())
	}
	requireLatestSchema(t)
}

func TestInitUpgradeDoesNotReplaceCA(t *testing.T) {
	initFreshBucket(t)

	caCertBefore, err := os.ReadFile(path.Join(bucket.SecretLocation, "ca.crt"))
	require.NoError(t, err)
	caKeyBefore, err := os.ReadFile(path.Join(bucket.SecretLocation, "ca.key"))
	require.NoError(t, err)

	upgradeBucket(t)

	caCertAfter, err := os.ReadFile(path.Join(bucket.SecretLocation, "ca.crt"))
	require.NoError(t, err)
	caKeyAfter, err := os.ReadFile(path.Join(bucket.SecretLocation, "ca.key"))
	require.NoError(t, err)

	assert.Equal(t, caCertBefore, caCertAfter)
	assert.Equal(t, caKeyBefore, caKeyAfter)
}

func TestInitWithWorkspaceButNoDatabase(t *testing.T) {
	resetBucket(t)

	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(
		path.Join(bucket.WorkspaceLocation, "workers.json"),
		[]byte(workerJSONFixture),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		path.Join(bucket.WorkspaceLocation, "bucket.conf"),
		[]byte(`debug="1"`),
		0o644,
	))

	require.NoError(t, initialize.Execute())
	require.True(t, data.DatabaseExists())
	requireLatestSchema(t)
	assert.NotEmpty(t, mustGetBucketID(t))
}

func TestInitDoesNotOverwriteExistingBucketConf(t *testing.T) {
	resetBucket(t)

	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	customConf := `custom="yes"`
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "bucket.conf"), []byte(customConf), 0o644))

	require.NoError(t, initialize.Execute())

	content, err := os.ReadFile(path.Join(bucket.WorkspaceLocation, "bucket.conf"))
	require.NoError(t, err)
	assert.Equal(t, customConf, string(content))
}

func TestInitCreatesTmpDirectory(t *testing.T) {
	initFreshBucket(t)
	assert.DirExists(t, bucket.TempLocation)
}

func TestInitIncompleteCAFails(t *testing.T) {
	resetBucket(t)

	require.NoError(t, os.MkdirAll(bucket.SecretLocation, 0o755))
	require.NoError(t, os.WriteFile(path.Join(bucket.SecretLocation, "ca.crt"), []byte("partial"), 0o644))

	err := initialize.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CA is incomplete")
}
