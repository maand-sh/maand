package deploy

import (
	"testing"

	"maand/data"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateFuncMap_getAndKeys(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	store.Put("vars/job/app", "name", "maand", 0)

	allowed := data.AllowedKVNamespaces("app", "10.0.0.1")
	funcs := templateFuncMap("app", allowed)
	get := funcs["get"].(func(string, string) string)
	keys := funcs["keys"].(func(string) []string)

	assert.Equal(t, "maand", get("vars/job/app", "name"))
	assert.Equal(t, []string{"name"}, keys("vars/job/app"))
	require.NoError(t, tx.Rollback())
}
