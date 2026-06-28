package data

import (
	"database/sql"
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromoteAllocationStateCopiesFileManifest(t *testing.T) {
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
	_, err = tx.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES (?, '10.0.0.1', 'api', 0, 0, 0, '1.0.0')`, allocID)
	require.NoError(t, err)

	current := FileManifest{"Makefile": "abc", "config/app.toml": "def"}
	require.NoError(t, UpdateAllocationPlan(tx, namespace, allocID, "hash-a", current))
	require.NoError(t, PromoteAllocationState(tx, namespace, allocID))

	manifests, err := GetAllocationFileManifests(tx, namespace, allocID)
	require.NoError(t, err)
	assert.True(t, manifests.HasPreviousFiles)
	assert.Equal(t, current, manifests.Previous)
}

func TestParseFileManifestNull(t *testing.T) {
	manifest, ok, err := ParseFileManifest(sql.NullString{})
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, manifest)
}
