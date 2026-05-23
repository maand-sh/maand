// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import (
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	root := t.TempDir()
	bucket.Location = root
	bucket.SecretLocation = path.Join(root, "secrets")
	require.NoError(t, os.MkdirAll(bucket.SecretLocation, 0o755))
	require.NoError(t, EnsureEncryptionKey())
	ResetEncryptionKeyCacheForTest()
	t.Cleanup(ResetEncryptionKeyCacheForTest)

	encrypted, err := EncryptPlaintext("super-secret-token")
	require.NoError(t, err)
	assert.True(t, IsEncryptedValue(encrypted))

	plaintext, err := DecryptStoredValue(encrypted)
	require.NoError(t, err)
	assert.Equal(t, "super-secret-token", plaintext)
}

func TestStorePutSecretGetSecret(t *testing.T) {
	root := t.TempDir()
	bucket.Location = root
	bucket.SecretLocation = path.Join(root, "secrets")
	require.NoError(t, os.MkdirAll(bucket.SecretLocation, 0o755))
	require.NoError(t, EnsureEncryptionKey())
	ResetEncryptionKeyCacheForTest()
	t.Cleanup(ResetEncryptionKeyCacheForTest)

	store := NewStore()
	ns := SecretJobNamespace("api")
	require.NoError(t, store.PutSecret(ns, "db_password", "p@ssw0rd", 0))

	entry, err := store.Get(ns, "db_password")
	require.NoError(t, err)
	assert.True(t, IsEncryptedValue(entry.Value))

	plaintext, err := store.GetSecret(ns, "db_password")
	require.NoError(t, err)
	assert.Equal(t, "p@ssw0rd", plaintext)
}
