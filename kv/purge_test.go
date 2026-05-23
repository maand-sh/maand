// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPurgeStaleVersionsRemovesOldRows(t *testing.T) {
	db := openTestDB(t)
	defer func() {
		_ = db.Close()
	}()

	oldTS := time.Now().Add(-48 * time.Hour).Unix()
	_, err := db.Exec(
		`INSERT INTO key_value (key, value, namespace, version, ttl, created_date, deleted) VALUES
		 ('k', 'v1', 'ns', 1, 0, ?, 0),
		 ('k', 'v2', 'ns', 2, 0, ?, 0),
		 ('k', 'v3', 'ns', 3, 0, ?, 0),
		 ('k', 'v4', 'ns', 4, 0, ?, 0),
		 ('k', 'v5', 'ns', 5, 0, ?, 0),
		 ('k', 'v6', 'ns', 6, 0, ?, 0),
		 ('k', 'v7', 'ns', 7, 0, ?, 0),
		 ('k', 'v8', 'ns', 8, 0, ?, 0),
		 ('k', 'v9', 'ns', 9, 0, ?, 0),
		 ('k', 'v10', 'ns', 10, 0, ?, 1)`,
		oldTS, oldTS, oldTS, oldTS, oldTS, oldTS, oldTS, oldTS, oldTS, oldTS,
	)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	store := NewStore()
	require.NoError(t, store.PurgeStaleVersions(tx, 0))
	require.NoError(t, tx.Commit())

	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM key_value WHERE namespace = 'ns' AND key = 'k'`).Scan(&count))
	assert.Less(t, count, 10)
}
