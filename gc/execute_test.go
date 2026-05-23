// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gc

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func TestPurgeRemovedAllocations(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:gctest?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	_, err = db.Exec(`
		CREATE TABLE allocations (alloc_id TEXT, worker_ip TEXT, job TEXT, disabled INT, removed INT, deployment_seq INT);
		CREATE TABLE hash (namespace TEXT, key TEXT, current_hash TEXT, previous_hash TEXT, PRIMARY KEY(namespace, key));
	`)
	require.NoError(t, err)

	_, err = db.Exec(
		`INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		 VALUES ('a1', '10.0.0.1', 'app', 0, 1, 0)`,
	)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO hash (namespace, key, current_hash) VALUES ('app_allocation', 'a1', 'hash')`,
	)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, purgeRemovedAllocations(tx))
	require.NoError(t, tx.Commit())

	var allocCount, hashCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM allocations`).Scan(&allocCount))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM hash WHERE namespace = 'app_allocation'`).Scan(&hashCount))
	assert.Equal(t, 0, allocCount)
	assert.Equal(t, 0, hashCount)
}
