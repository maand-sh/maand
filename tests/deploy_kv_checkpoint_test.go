package tests

import (
	"testing"

	"maand/data"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistSessionSurvivesDeployTransactionRollback(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	runBuild(t)

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() {
		_ = tx.Rollback()
	}()

	require.NoError(t, kv.Initialize(tx))

	store, err := kv.RequireStore()
	require.NoError(t, err)
	store.Put("vars/job/demo", "deploy_marker", "checkpoint-value", 0)

	require.NoError(t, kv.PersistSession())
	require.NoError(t, tx.Rollback())

	value, err := GetKey("vars/job/demo", "deploy_marker")
	require.NoError(t, err)
	assert.Equal(t, "checkpoint-value", value)
}
