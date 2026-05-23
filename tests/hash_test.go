// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"testing"

	"maand/data"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashLifecycle(t *testing.T) {
	initFreshBucket(t)

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

	require.NoError(t, data.UpdateHash(tx, "test", "item", "hash-a"))
	changed, err := data.HashChanged(tx, "test", "item")
	require.NoError(t, err)
	assert.True(t, changed)

	require.NoError(t, data.PromoteHash(tx, "test", "item"))
	changed, err = data.HashChanged(tx, "test", "item")
	require.NoError(t, err)
	assert.False(t, changed)

	require.NoError(t, data.UpdateHash(tx, "test", "item", "hash-b"))
	changed, err = data.HashChanged(tx, "test", "item")
	require.NoError(t, err)
	assert.True(t, changed)

	previous, err := data.GetPreviousHash(tx, "test", "item")
	require.NoError(t, err)
	assert.Equal(t, "hash-a", previous)

	require.NoError(t, tx.Commit())
}
