package deploy

import (
	"testing"

	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistJobCommandKV_persistsPendingChanges(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	store.Put("vars/job/app", "marker", "pending", 0)
	require.NoError(t, persistJobCommandKV(tx, "app"))
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	value, err := kv.GetKVStore().Get("vars/job/app", "marker")
	require.NoError(t, err)
	assert.Equal(t, "pending", value.Value)
	require.NoError(t, tx.Rollback())
}

func TestPersistJobCommandKV_noPendingChanges(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, persistJobCommandKV(tx, "app"))
	require.NoError(t, tx.Rollback())
}
