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

func TestMigrateSchemaCreatesCatalogViews(t *testing.T) {
	initFreshBucket(t)

	views := []string{
		"cat_allocations",
		"cat_jobs",
		"cat_job_commands",
		"cat_kv",
		"cat_workers",
	}
	for _, viewName := range views {
		assert.True(t, mustViewExists(t, viewName), "missing view %s", viewName)
	}
}

func TestMigrateSchemaIdempotentInTransaction(t *testing.T) {
	initFreshBucket(t)
	versionBefore := mustGetSchemaVersion(t)

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

	require.NoError(t, data.MigrateSchema(tx))
	require.NoError(t, tx.Commit())

	assert.Equal(t, versionBefore, mustGetSchemaVersion(t))
}

func TestCatJobsViewQueryableAfterUpgrade(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeMinimalJob(t, "demo", `{"selectors":["worker"]}`)

	runBuild(t)

	count := MustQueryCount(t, `SELECT count(*) FROM cat_jobs WHERE name = 'demo'`)
	assert.Equal(t, 1, count)
}
