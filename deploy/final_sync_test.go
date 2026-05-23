package deploy

import (
	"database/sql"
	"strings"
	"testing"

	"maand/data"

	_ "github.com/mattn/go-sqlite3"
)

func openFinalSyncTestDB(t *testing.T) *sql.DB {
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

func TestFinalSyncWorkersForJob_activeOnly(t *testing.T) {
	db := openFinalSyncTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, row := range []struct {
		allocID, workerIP, job string
		removed              int
	}{
		{"a1", "10.0.0.1", "api", 0},
		{"a2", "10.0.0.2", "api", 1},
	} {
		_, err := tx.Exec(
			`INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
			 VALUES (?, ?, ?, 0, ?, 0)`,
			row.allocID, row.workerIP, row.job, row.removed,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	workers, err := data.GetActiveAllocations(tx, "api")
	if err != nil {
		t.Fatal(err)
	}
	if len(workers) != 1 || workers[0] != "10.0.0.1" {
		t.Fatalf("final sync should target only active workers: %#v", workers)
	}
}

func TestDeployedJobsFilterPerJobRsyncRules(t *testing.T) {
	apiRules := strings.Join(buildRsyncFilterLines([]string{"api"}, true), "")
	workerRules := strings.Join(buildRsyncFilterLines([]string{"worker"}, true), "")
	if apiRules == workerRules {
		t.Fatal("each deployed job should have its own rsync include rules")
	}
	if !strings.Contains(apiRules, "+ jobs/api/") || strings.Contains(apiRules, "+ jobs/worker/") {
		t.Fatalf("api rules should only include api: %q", apiRules)
	}
	if !strings.Contains(workerRules, "+ jobs/worker/") || strings.Contains(workerRules, "+ jobs/api/") {
		t.Fatalf("worker rules should only include worker: %q", workerRules)
	}
}
