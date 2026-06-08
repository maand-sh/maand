// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorePutGetAndVersioning(t *testing.T) {
	store := NewStore()

	store.Put("ns", "key", "v1", 0)
	entry, err := store.Get("ns", "key")
	require.NoError(t, err)
	assert.Equal(t, "v1", entry.Value)
	assert.Equal(t, 1, entry.Version)
	assert.True(t, entry.Changed)

	store.Put("ns", "key", "v2", 0)
	entry, err = store.Get("ns", "key")
	require.NoError(t, err)
	assert.Equal(t, "v2", entry.Value)
	assert.Equal(t, 2, entry.Version)
}

func TestStoreDeleteMarksDeleted(t *testing.T) {
	store := NewStore()
	store.Put("ns", "key", "v", 0)

	require.NoError(t, store.Delete("ns", "key"))
	_, err := store.Get("ns", "key")
	assert.ErrorIs(t, err, ErrNotFound)

	keys, err := store.GetKeys("ns")
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestStoreReviveDeletedKey(t *testing.T) {
	store := NewStore()
	store.Put("ns", "key", "old", 0)
	require.NoError(t, store.Delete("ns", "key"))

	store.Put("ns", "key", "new", 0)
	entry, err := store.Get("ns", "key")
	require.NoError(t, err)
	assert.Equal(t, "new", entry.Value)
	assert.Equal(t, 3, entry.Version)
}

func TestStoreTrimsWhitespace(t *testing.T) {
	store := NewStore()
	store.Put("ns", "key", "  spaced  ", 0)
	entry, err := store.Get("ns", "key")
	require.NoError(t, err)
	assert.Equal(t, "spaced", entry.Value)
}

func TestRequireStoreWithoutInitialize(t *testing.T) {
	sessionStore = nil
	_, err := RequireStore()
	assert.ErrorIs(t, err, ErrStoreNotInitialized)
}
