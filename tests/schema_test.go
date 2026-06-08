// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"testing"

	"maand/bucket"
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
		"cat_hashes",
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

func TestUpgradeBucketFromV1ToV2(t *testing.T) {
	initFreshBucket(t)
	requireLatestSchema(t)
	require.True(t, mustViewExists(t, "cat_hashes"))

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE schema_version SET version = 1`)
	require.NoError(t, err)
	_, err = db.Exec(`DROP VIEW IF EXISTS cat_hashes`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	err = data.CheckSchemaVersion()
	require.ErrorIs(t, err, bucket.ErrSchemaUpgradeRequired)

	upgradeBucket(t)

	requireLatestSchema(t)
	assert.True(t, mustViewExists(t, "cat_hashes"))
	require.NoError(t, data.CheckSchemaVersion())
}

func TestCatJobsViewQueryableAfterUpgrade(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeMinimalJob(t, "demo", `{"selectors":["worker"]}`)

	runBuild(t)

	count := MustQueryCount(t, `SELECT count(*) FROM cat_jobs WHERE name = 'demo'`)
	assert.Equal(t, 1, count)
}
