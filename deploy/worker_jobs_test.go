package deploy

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openWorkerJobsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE allocations (
		alloc_id TEXT, worker_ip TEXT, job TEXT,
		disabled INT, removed INT, deployment_seq INT,
		PRIMARY KEY(worker_ip, job)
	)`); err != nil {
		t.Fatal(err)
	}
	return db
}

func insertWorkerJobAllocation(t *testing.T, tx *sql.Tx, allocID, workerIP, job string, disabled, removed int) {
	t.Helper()
	_, err := tx.Exec(
		`INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		 VALUES (?, ?, ?, ?, ?, 0)`,
		allocID, workerIP, job, disabled, removed,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildWorkerJobsList_excludesRemoved(t *testing.T) {
	db := openWorkerJobsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	insertWorkerJobAllocation(t, tx, "a1", "10.0.0.1", "api", 0, 0)
	insertWorkerJobAllocation(t, tx, "a2", "10.0.0.1", "legacy", 0, 1)

	got, err := buildWorkerJobsList(tx, "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Job != "api" {
		t.Fatalf("got %#v", got)
	}
}

func TestBuildWorkerJobsList_includesDisabled(t *testing.T) {
	db := openWorkerJobsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	insertWorkerJobAllocation(t, tx, "a1", "10.0.0.1", "api", 1, 0)

	got, err := buildWorkerJobsList(tx, "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Disabled != 1 {
		t.Fatalf("got %#v", got)
	}
}
