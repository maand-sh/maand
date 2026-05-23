package kv

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/data"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistSessionCommitsIndependentOfCallerTx(t *testing.T) {
	dir := t.TempDir()
	orig := bucket.Location
	bucket.Location = dir
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, os.MkdirAll(path.Join(dir, "data"), 0o755))

	db, err := data.OpenDatabase(false)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	setupTx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, data.MigrateSchema(setupTx))
	require.NoError(t, setupTx.Commit())

	callerTx, err := db.Begin()
	require.NoError(t, err)

	store, err := LoadFromTransaction(callerTx)
	require.NoError(t, err)
	sessionStore = store

	store.Put("ns", "k", "from-job-command", 0)
	require.True(t, store.HasPendingChanges())
	require.NoError(t, callerTx.Rollback())

	require.NoError(t, PersistSession())

	var value string
	require.NoError(t, db.QueryRow(
		`SELECT value FROM key_value WHERE namespace = 'ns' AND key = 'k' ORDER BY version DESC LIMIT 1`,
	).Scan(&value))
	assert.Equal(t, "from-job-command", value)
}

func TestPersistToSessionTransactionUsesOpenTx(t *testing.T) {
	dir := t.TempDir()
	orig := bucket.Location
	bucket.Location = dir
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, os.MkdirAll(path.Join(dir, "data"), 0o755))

	db, err := data.OpenDatabase(false)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	setupTx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, data.MigrateSchema(setupTx))
	require.NoError(t, setupTx.Commit())

	tx, err := db.Begin()
	require.NoError(t, err)

	store, err := LoadFromTransaction(tx)
	require.NoError(t, err)
	sessionStore = store

	store.Put("ns", "k", "in-deploy-tx", 0)
	require.NoError(t, PersistToSessionTransaction(tx))
	require.False(t, store.HasPendingChanges())
	require.NoError(t, tx.Commit())

	var value string
	require.NoError(t, db.QueryRow(
		`SELECT value FROM key_value WHERE namespace = 'ns' AND key = 'k' ORDER BY version DESC LIMIT 1`,
	).Scan(&value))
	assert.Equal(t, "in-deploy-tx", value)
}

func TestHasPendingChanges(t *testing.T) {
	store := NewStore()
	assert.False(t, store.HasPendingChanges())
	store.Put("ns", "k", "v", 0)
	assert.True(t, store.HasPendingChanges())
}
