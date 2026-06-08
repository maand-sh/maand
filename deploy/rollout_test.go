package deploy

import (
	"database/sql"
	"testing"

	"maand/data"

	_ "github.com/mattn/go-sqlite3"
)

func openRolloutTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE allocations (
			alloc_id TEXT, worker_ip TEXT, job TEXT,
			disabled INT, removed INT, deployment_seq INT,
			new_version TEXT,
			PRIMARY KEY(worker_ip, job)
		)`,
		`CREATE TABLE hash (
			namespace TEXT, key TEXT,
			current_hash TEXT, previous_hash TEXT,
			current_version TEXT,
			PRIMARY KEY(namespace, key)
		)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func insertAllocation(t *testing.T, tx *sql.Tx, allocID, workerIP, job string) {
	t.Helper()
	_, err := tx.Exec(
		`INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		 VALUES (?, ?, ?, 0, 0, 0, '0.0.0')`,
		allocID, workerIP, job,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestJobNeedsRollout_newAllocation(t *testing.T) {
	db := openRolloutTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	insertAllocation(t, tx, "alloc-1", "10.0.0.1", "api")
	if err := data.UpdateHash(tx, "api_allocation", "alloc-1", "hash-a"); err != nil {
		t.Fatal(err)
	}

	needs, err := JobNeedsRollout(tx, "api")
	if err != nil {
		t.Fatal(err)
	}
	if !needs {
		t.Fatal("expected rollout for new allocation")
	}
}

func TestJobNeedsRollout_noHashRow(t *testing.T) {
	db := openRolloutTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	insertAllocation(t, tx, "alloc-1", "10.0.0.1", "api")

	needs, err := JobNeedsRollout(tx, "api")
	if err != nil {
		t.Fatal(err)
	}
	if !needs {
		t.Fatal("expected rollout when allocation has no hash row yet")
	}
}

func TestJobNeedsRollout_promoted(t *testing.T) {
	db := openRolloutTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	insertAllocation(t, tx, "alloc-1", "10.0.0.1", "api")
	if err := data.UpdateHash(tx, "api_allocation", "alloc-1", "hash-a"); err != nil {
		t.Fatal(err)
	}
	if err := data.PromoteHash(tx, "api_allocation", "alloc-1"); err != nil {
		t.Fatal(err)
	}

	needs, err := JobNeedsRollout(tx, "api")
	if err != nil {
		t.Fatal(err)
	}
	if needs {
		t.Fatal("expected no rollout after promote")
	}
}

func TestJobNeedsRollout_pendingRestart(t *testing.T) {
	db := openRolloutTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	insertAllocation(t, tx, "alloc-1", "10.0.0.1", "api")
	if err := data.UpdateHash(tx, "api_allocation", "alloc-1", "hash-a"); err != nil {
		t.Fatal(err)
	}
	if err := data.PromoteHash(tx, "api_allocation", "alloc-1"); err != nil {
		t.Fatal(err)
	}
	if err := data.UpdateHash(tx, "api_allocation", "alloc-1", "hash-b"); err != nil {
		t.Fatal(err)
	}

	needs, err := JobNeedsRollout(tx, "api")
	if err != nil {
		t.Fatal(err)
	}
	if !needs {
		t.Fatal("expected rollout when current_hash differs from previous_hash")
	}
}

func TestJobNeedsRollout_versionMismatch(t *testing.T) {
	db := openRolloutTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	insertAllocation(t, tx, "alloc-1", "10.0.0.1", "api")
	if err := data.UpdateHash(tx, "api_allocation", "alloc-1", "hash-a"); err != nil {
		t.Fatal(err)
	}
	if err := data.PromoteHash(tx, "api_allocation", "alloc-1"); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(`UPDATE hash SET current_version = '1.0.0' WHERE namespace = 'api_allocation' AND key = 'alloc-1'`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(`UPDATE allocations SET new_version = '2.0.0' WHERE alloc_id = 'alloc-1'`)
	if err != nil {
		t.Fatal(err)
	}

	needs, err := JobNeedsRollout(tx, "api")
	if err != nil {
		t.Fatal(err)
	}
	if !needs {
		t.Fatal("expected rollout when current_version differs from allocation new_version")
	}
}
