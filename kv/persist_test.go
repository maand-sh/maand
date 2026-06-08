// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:kvtest?mode=memory&cache=shared")
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE key_value (
		key TEXT, value TEXT, namespace TEXT, version INT,
		ttl TEXT, created_date TEXT, deleted INT
	)`)
	require.NoError(t, err)
	return db
}

func TestLoadAndPersistRoundTrip(t *testing.T) {
	db := openTestDB(t)
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	require.NoError(t, err)

	_, err = tx.Exec(
		`INSERT INTO key_value (key, value, namespace, version, ttl, created_date, deleted)
		 VALUES ('k', 'v1', 'ns', 1, 0, 100, 0)`,
	)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	store, err := LoadFromTransaction(tx)
	require.NoError(t, err)

	entry, err := store.Get("ns", "k")
	require.NoError(t, err)
	assert.Equal(t, "v1", entry.Value)
	assert.False(t, entry.Changed)

	store.Put("ns", "k", "v2", 0)
	require.NoError(t, PersistToTransaction(tx, store))
	require.NoError(t, tx.Commit())

	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM key_value WHERE namespace = 'ns' AND key = 'k'`).Scan(&count))
	assert.Equal(t, 2, count)

	var latestValue string
	require.NoError(t, db.QueryRow(
		`SELECT value FROM key_value WHERE namespace = 'ns' AND key = 'k' ORDER BY version DESC LIMIT 1`,
	).Scan(&latestValue))
	assert.Equal(t, "v2", latestValue)
}

func TestLoadReturnsLatestVersionOnly(t *testing.T) {
	db := openTestDB(t)
	defer func() {
		_ = db.Close()
	}()

	_, err := db.Exec(
		`INSERT INTO key_value (key, value, namespace, version, ttl, created_date, deleted) VALUES
		 ('k', 'old', 'ns', 1, 0, 1, 0),
		 ('k', 'new', 'ns', 2, 0, 2, 0)`,
	)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	store, err := LoadFromTransaction(tx)
	require.NoError(t, err)
	_ = tx.Rollback()

	entry, err := store.Get("ns", "k")
	require.NoError(t, err)
	assert.Equal(t, "new", entry.Value)
	assert.Equal(t, 2, entry.Version)
}

func TestLoadAfterReviveDeletedKeepsNewest(t *testing.T) {
	db := openTestDB(t)
	defer func() {
		_ = db.Close()
	}()

	_, err := db.Exec(
		`INSERT INTO key_value (key, value, namespace, version, ttl, created_date, deleted) VALUES
		 ('k', 'old', 'ns', 1, 0, 1, 0),
		 ('k', 'old', 'ns', 2, 0, 2, 1),
		 ('k', 'new', 'ns', 3, 0, 3, 0)`,
	)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	store, err := LoadFromTransaction(tx)
	require.NoError(t, err)
	_ = tx.Rollback()

	entry, err := store.Get("ns", "k")
	require.NoError(t, err)
	assert.Equal(t, "new", entry.Value)
	assert.Equal(t, 3, entry.Version)
}
