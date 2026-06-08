// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"os"
	"path"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseExistsAndOpen(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	assert.False(t, DatabaseExists())

	_, err := OpenDatabase(true)
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrNotInitialized)

	require.NoError(t, os.MkdirAll(path.Join(root, "data"), 0o755))
	fileDB, err := OpenDatabase(false)
	require.NoError(t, err)
	tx, err := fileDB.Begin()
	require.NoError(t, err)
	require.NoError(t, MigrateSchema(tx))
	require.NoError(t, tx.Commit())
	require.NoError(t, fileDB.Close())

	assert.True(t, DatabaseExists())

	opened, err := OpenDatabase(true)
	require.NoError(t, err)
	require.NoError(t, opened.Close())
}

func TestCopyJobFilesAndCommandModule(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	_, err = tx.Exec(`
		INSERT INTO job_files (job_id, path, content, isdir)
		VALUES ('job-api', 'api/Makefile', '', 0),
		       ('job-api', 'api/data', '', 1);
	`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	outDir := t.TempDir()

	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, CopyJobFiles(tx, "api", outDir))
	require.NoError(t, tx.Rollback())

	_, err = os.Stat(path.Join(outDir, "api", "Makefile"))
	require.NoError(t, err)
	_, err = os.Stat(path.Join(outDir, "api", "data"))
	require.NoError(t, err)
}

func TestCopyJobCommandModuleWritesModuleFiles(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	_, err = tx.Exec(`
		INSERT INTO job_files (job_id, path, content, isdir)
		VALUES ('job-api', 'api/_modules/init.sh', '#!/bin/sh', 0);
	`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	outDir := t.TempDir()
	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, CopyJobCommandModule(tx, "api", "init", outDir))
	require.NoError(t, tx.Rollback())

	_, err = os.Stat(path.Join(outDir, "api", "_modules", "init.sh"))
	require.NoError(t, err)
}

func TestCopyJobCommandModuleRequiresFiles(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Rollback())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = CopyJobCommandModule(tx, "api", "init", t.TempDir())
	require.Error(t, err)
}
