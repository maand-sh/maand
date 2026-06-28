// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
	"maand/data"
)

func TestResolveJobsForCommand_singleJob(t *testing.T) {
	db := openExecuteTestDB(t)
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	seedJobCommand(t, tx, "api", "command_status", "cli")

	jobs, err := resolveJobsForCommand(tx, "api", "command_status", "cli")
	require.NoError(t, err)
	assert.Equal(t, []string{"api"}, jobs)
}

func TestResolveJobsForCommand_allJobs(t *testing.T) {
	db := openExecuteTestDB(t)
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	seedJobCommand(t, tx, "api", "command_status", "cli")
	seedJobCommand(t, tx, "db", "command_status", "cli")
	seedJobCommand(t, tx, "cache", "command_other", "cli")

	jobs, err := resolveJobsForCommand(tx, "", "command_status", "cli")
	require.NoError(t, err)
	assert.Equal(t, []string{"api", "db"}, jobs)
}

func TestResolveJobsForCommand_notFound(t *testing.T) {
	db := openExecuteTestDB(t)
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	_, err = resolveJobsForCommand(tx, "", "missing", "cli")
	require.Error(t, err)
	var notFound *NotFoundError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, "missing", notFound.Command)
}

func openExecuteTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, data.MigrateSchema(tx))
	require.NoError(t, tx.Commit())
	return db
}

func seedJobCommand(t *testing.T, tx *sql.Tx, job, command, event string) {
	t.Helper()
	jobID := "job-" + job
	_, err := tx.Exec(`
		INSERT OR REPLACE INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES (?, ?, '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
		VALUES (?, ?, ?, ?, '', '', '');`,
		jobID, job, jobID, job, command, event,
	)
	require.NoError(t, err)
}
