package deploy

import (
	"testing"

	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRolloutOrder_usesKVWhenValid(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.insertAllocation(t, tx, "a2", "10.0.0.2", "app", 0, 0, 1)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store := kv.GetKVStore()
	store.Put("maand/job/app", "rollout_order", "10.0.0.2,10.0.0.1", 0)

	resolved, err := ResolveRolloutOrder(tx, "app", []string{"10.0.0.1", "10.0.0.2"})
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.2", "10.0.0.1"}, resolved.Ordered)
	assert.Equal(t, orderSourceKV, resolved.Source)
	require.NoError(t, tx.Rollback())
}

func TestResolveRolloutOrder_fallbackOnMismatch(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	env.seedMakefileJob(t, tx, "app", "10.0.0.1", 0)
	env.ensureWorker(t, tx, "10.0.0.2", 1)
	env.insertAllocation(t, tx, "a2", "10.0.0.2", "app", 0, 0, 1)
	require.NoError(t, tx.Commit())

	tx = env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store := kv.GetKVStore()
	store.Put("maand/job/app", "rollout_order", "10.0.0.1", 0)

	resolved, err := ResolveRolloutOrder(tx, "app", []string{"10.0.0.1", "10.0.0.2"})
	require.NoError(t, err)
	assert.Equal(t, orderSourceDefault, resolved.Source)
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, resolved.Ordered)
	require.NoError(t, tx.Rollback())
}

func TestBatchEnv(t *testing.T) {
	env := batchEnv(BatchContext{
		Job:             "app",
		Phase:           deployPhaseNew,
		BatchIndex:      1,
		BatchCount:      3,
		BatchAllocation: []string{"10.0.0.2"},
		RolloutOrder:     "10.0.0.2,10.0.0.1",
		OrderSource:     orderSourceKV,
	})
	assert.Contains(t, env, "BATCH_ALLOCATIONS=10.0.0.2")
	assert.Contains(t, env, "BATCH_INDEX=1")
	assert.Contains(t, env, "DEPLOY_PHASE=new")
	assert.Contains(t, env, "ROLLOUT_ORDER_SOURCE=kv")
}
