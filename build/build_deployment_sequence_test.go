// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"maand/bucket"
)

func TestDetectCircularJobCommandDependencies_cycle(t *testing.T) {
	err := detectCircularJobCommandDependencies(map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrCircularJobCommandDependency)
}

func TestDetectCircularJobCommandDependencies_acyclic(t *testing.T) {
	err := detectCircularJobCommandDependencies(map[string][]string{
		"consumer": {"db"},
		"db":       nil,
	})
	require.NoError(t, err)
}

func TestLoadJobCommandDependencies(t *testing.T) {
	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO job (job_id, name, version, min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz, update_parallel_count, health_check)
		VALUES ('job-db', 'db', '1.0.0', '0', '0', '0', '0', '0', '0', 1, ''),
		       ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
		VALUES ('job-api', 'api', 'command_migrate', 'cli', 'db', 'command_schema', '{}');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	deps, err := loadJobCommandDependencies(tx)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())

	assert.Equal(t, []string{"db"}, deps["api"])
	assert.Nil(t, deps["db"])
}

func TestBuildDeploymentSequence_ordersDependentJobs(t *testing.T) {
	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO job (job_id, name, version, min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz, update_parallel_count, health_check)
		VALUES ('job-db', 'db', '1.0.0', '0', '0', '0', '0', '0', '0', 1, ''),
		       ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
		VALUES ('job-db', 'db', 'command_schema', 'cli', '', '', '{}'),
		       ('job-api', 'api', 'command_migrate', 'cli', 'db', 'command_schema', '{}');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a-db', '10.0.0.1', 'db', 0, 0, 0, '0.0.0'),
		       ('a-api', '10.0.0.1', 'api', 0, 0, 0, '0.0.0');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, BuildDeploymentSequence(tx))
	require.NoError(t, tx.Commit())

	var dbSeq, apiSeq int
	require.NoError(t, db.QueryRow(`SELECT deployment_seq FROM allocations WHERE job = 'db'`).Scan(&dbSeq))
	require.NoError(t, db.QueryRow(`SELECT deployment_seq FROM allocations WHERE job = 'api'`).Scan(&apiSeq))
	assert.Less(t, dbSeq, apiSeq)
}

func TestBuildDeploymentSequence_rejectsCycle(t *testing.T) {
	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO job (job_id, name, version, min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz, update_parallel_count, health_check)
		VALUES ('job-a', 'a', '1.0.0', '0', '0', '0', '0', '0', '0', 1, ''),
		       ('job-b', 'b', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
		VALUES ('job-a', 'a', 'command_a', 'cli', 'b', 'command_b', '{}'),
		       ('job-b', 'b', 'command_b', 'cli', 'a', 'command_a', '{}');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	err = BuildDeploymentSequence(tx)
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrCircularJobCommandDependency)
	require.NoError(t, tx.Rollback())
}
