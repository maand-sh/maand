package data

import (
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeDeployVersion(t *testing.T) {
	assert.Equal(t, "0.0.0", NormalizeDeployVersion(""))
	assert.Equal(t, "0.0.0", NormalizeDeployVersion("unknown"))
	assert.Equal(t, "1.2.3", NormalizeDeployVersion("1.2.3"))
}

func TestAllocationVersionLifecycle(t *testing.T) {
	root := t.TempDir()
	oldLocation := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = oldLocation
		bucket.UpdatePath()
	})
	require.NoError(t, os.MkdirAll(path.Join(root, "data"), 0o755))

	db, err := OpenDatabase(false)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, MigrateSchema(tx))
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tx.Rollback())
	}()

	const namespace = "api_allocation"
	const allocID = "alloc-1"

	versions, err := GetAllocationVersions(tx, namespace, allocID)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0", versions.CurrentVersion)
	assert.Equal(t, "0.0.0", versions.NewVersion)

	require.NoError(t, UpdateAllocationPlan(tx, namespace, allocID, "hash-a", "2.0.0"))
	versions, err = GetAllocationVersions(tx, namespace, allocID)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0", versions.CurrentVersion)
	assert.Equal(t, "2.0.0", versions.NewVersion)

	require.NoError(t, PromoteAllocationState(tx, namespace, allocID))
	versions, err = GetAllocationVersions(tx, namespace, allocID)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", versions.CurrentVersion)
	assert.Equal(t, "2.0.0", versions.NewVersion)

	require.NoError(t, UpdateAllocationPlan(tx, namespace, allocID, "hash-b", "2.1.0"))
	versions, err = GetAllocationVersions(tx, namespace, allocID)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", versions.CurrentVersion)
	assert.Equal(t, "2.1.0", versions.NewVersion)

	require.NoError(t, ClearAllocationLiveState(tx, namespace, allocID))
	versions, err = GetAllocationVersions(tx, namespace, allocID)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0", versions.CurrentVersion)
}
