// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"os"
	"strings"
	"testing"
	"text/template"

	"maand/bucket"
	"maand/data"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateGetSecret(t *testing.T) {
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

	tpl := `token={{ getSecret "token" }}`
	funcMap := templateFuncMap(tx, "app", data.AllowedKVNamespaces("app", "10.0.0.1"))
	tmpl, err := template.New("test").Funcs(funcMap).Parse(tpl)
	require.NoError(t, err)

	var buf strings.Builder
	require.NoError(t, tmpl.Execute(&buf, AllocationData{}))
	assert.Equal(t, "token=abc123", buf.String())
	require.NoError(t, tx.Rollback())
}

func TestTemplateGetDecryptsSecretsNamespace(t *testing.T) {
	env := setupDeployTestEnv(t)
	require.NoError(t, os.MkdirAll(bucket.SecretLocation, 0o755))
	require.NoError(t, kv.EnsureEncryptionKey())
	kv.ResetEncryptionKeyCacheForTest()
	t.Cleanup(kv.ResetEncryptionKeyCacheForTest)

	tx := env.begin(t)
	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	require.NoError(t, store.PutSecret(kv.SecretJobNamespace("app"), "token", "secret-value", 0))

	ns := kv.SecretJobNamespace("app")
	tpl := `{{ get "` + ns + `" "token" }}`
	funcMap := templateFuncMap(tx, "app", data.AllowedKVNamespaces("app", "10.0.0.1"))
	tmpl, err := template.New("test").Funcs(funcMap).Parse(tpl)
	require.NoError(t, err)

	var buf strings.Builder
	require.NoError(t, tmpl.Execute(&buf, AllocationData{}))
	assert.Equal(t, "secret-value", buf.String())
	require.NoError(t, tx.Rollback())
}
