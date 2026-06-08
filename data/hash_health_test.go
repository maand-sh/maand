package data

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openHashHealthTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	for _, ddl := range []string{
		`CREATE TABLE allocations (
			alloc_id TEXT, worker_ip TEXT, job TEXT,
			disabled INT, removed INT, deployment_seq INT,
			PRIMARY KEY(worker_ip, job)
		)`,
		`CREATE TABLE hash (
			namespace TEXT, key TEXT,
			current_hash TEXT, previous_hash TEXT,
			PRIMARY KEY(namespace, key)
		)`,
	} {
		_, err := db.Exec(ddl)
		require.NoError(t, err)
	}
	return db
}

func TestMarkAllocationHealthFailedMarksPromotedAllocation(t *testing.T) {
	db := openHashHealthTestDB(t)
	defer db.Close()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(
		`INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		 VALUES ('alloc-1', '10.0.0.1', 'vault', 0, 0, 0)`,
	)
	require.NoError(t, err)
	_, err = tx.Exec(
		`INSERT INTO hash (namespace, key, current_hash, previous_hash)
		 VALUES ('vault_allocation', 'alloc-1', 'hash-live', 'hash-live')`,
	)
	require.NoError(t, err)

	require.NoError(t, MarkAllocationHealthFailed(tx, "vault", "10.0.0.1"))

	updated, err := GetUpdatedAllocations(tx, "vault")
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1"}, updated)
}

func TestMarkAllocationHealthFailedNoOpWhenNoHashRow(t *testing.T) {
	db := openHashHealthTestDB(t)
	defer db.Close()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(
		`INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		 VALUES ('alloc-1', '10.0.0.1', 'vault', 0, 0, 0)`,
	)
	require.NoError(t, err)

	require.NoError(t, MarkAllocationHealthFailed(tx, "vault", "10.0.0.1"))
}

func TestMarkAllocationHealthFailedNoOpWhenAlreadyOutOfSync(t *testing.T) {
	db := openHashHealthTestDB(t)
	defer db.Close()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(
		`INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		 VALUES ('alloc-1', '10.0.0.1', 'vault', 0, 0, 0)`,
	)
	require.NoError(t, err)
	_, err = tx.Exec(
		`INSERT INTO hash (namespace, key, current_hash, previous_hash)
		 VALUES ('vault_allocation', 'alloc-1', 'hash-new', 'hash-old')`,
	)
	require.NoError(t, err)

	require.NoError(t, MarkAllocationHealthFailed(tx, "vault", "10.0.0.1"))

	var previousHash string
	require.NoError(t, tx.QueryRow(
		`SELECT previous_hash FROM hash WHERE namespace = 'vault_allocation' AND key = 'alloc-1'`,
	).Scan(&previousHash))
	assert.Equal(t, "hash-old", previousHash)
}
