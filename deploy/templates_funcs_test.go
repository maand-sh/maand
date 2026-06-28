package deploy

import (
	"os"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateFuncMap_getSecretAndHelpers(t *testing.T) {
	env := setupDeployTestEnv(t)
	require.NoError(t, os.MkdirAll(bucket.SecretLocation, 0o755))
	require.NoError(t, kv.EnsureEncryptionKey())
	kv.ResetEncryptionKeyCacheForTest()
	t.Cleanup(kv.ResetEncryptionKeyCacheForTest)

	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	require.NoError(t, store.PutSecret(kv.SecretJobNamespace("app"), "token", "abc123", 0))

	allowed := data.AllowedKVNamespaces("app", "10.0.0.1")
	funcs := templateFuncMap(tx, "app", allowed)

	getSecret := funcs["getSecret"].(func(string) string)
	assert.Equal(t, "abc123", getSecret("token"))

	assert.Equal(t, 3, funcs["add"].(func(int, int) int)(1, 2))
	assert.Equal(t, "HELLO", funcs["upper"].(func(string) string)("hello"))
	assert.Equal(t, 42, funcs["int"].(func(any) int)("42"))
	require.NoError(t, tx.Rollback())
}

func TestTemplateFuncMap_mathAndStringHelpers(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))

	allowed := data.AllowedKVNamespaces("app", "10.0.0.1")
	funcs := templateFuncMap(tx, "app", allowed)

	assert.Equal(t, 7, funcs["add"].(func(int, int) int)(3, 4))
	assert.Equal(t, 1, funcs["sub"].(func(int, int) int)(3, 2))
	assert.Equal(t, 12, funcs["mul"].(func(int, int) int)(3, 4))
	assert.Equal(t, 2, funcs["div"].(func(int, int) int)(8, 4))
	assert.Equal(t, 3, funcs["min"].(func(int, int) int)(3, 8))
	assert.Equal(t, 8, funcs["max"].(func(int, int) int)(3, 8))
	assert.Equal(t, []string{"a", "b"}, funcs["split"].(func(string, string) []string)("a,b", ","))
	assert.Equal(t, "host", funcs["trim"].(func(string) string)("  host  "))
	assert.Equal(t, "hello", funcs["lower"].(func(string) string)("HELLO"))
	assert.Equal(t, "a-b", funcs["join"].(func([]string, string) string)([]string{"a", "b"}, "-"))
	assert.Equal(t, 99, funcs["int"].(func(any) int)(99))
	require.NoError(t, tx.Rollback())
}

func TestTemplateFuncMap_getPanicsOnDisallowedNamespace(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))

	allowed := data.AllowedKVNamespaces("app", "10.0.0.1")
	funcs := templateFuncMap(tx, "app", allowed)
	get := funcs["get"].(func(string, string) string)

	require.Panics(t, func() {
		_ = get("vars/job/other", "name")
	})
	require.NoError(t, tx.Rollback())
}

func TestTemplateFuncMap_getAndKeys(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	store.Put("vars/job/app", "name", "maand", 0)

	allowed := data.AllowedKVNamespaces("app", "10.0.0.1")
	funcs := templateFuncMap(tx, "app", allowed)
	get := funcs["get"].(func(string, string) string)
	keys := funcs["keys"].(func(string) []string)

	assert.Equal(t, "maand", get("vars/job/app", "name"))
	assert.Equal(t, []string{"name"}, keys("vars/job/app"))
	require.NoError(t, tx.Rollback())
}

func TestTemplateFuncMap_getOptional(t *testing.T) {
	env := setupDeployTestEnv(t)
	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	store.Put("vars/job/app", "name", "maand", 0)

	allowed := data.AllowedKVNamespaces("app", "10.0.0.1")
	funcs := templateFuncMap(tx, "app", allowed)
	getOptional := funcs["getOptional"].(func(string, string) string)

	assert.Equal(t, "maand", getOptional("vars/job/app", "name"))
	assert.Empty(t, getOptional("vars/job/app", "missing"))
	require.Panics(t, func() {
		_ = getOptional("vars/job/other", "name")
	})
	require.NoError(t, tx.Rollback())
}
