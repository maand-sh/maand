package deploy

import (
	"testing"

	"maand/kv"

	"github.com/stretchr/testify/require"
)

func TestPersistJobCommandKV_noPendingChanges(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, persistJobCommandKV(tx, "app"))
	require.NoError(t, tx.Rollback())
}
