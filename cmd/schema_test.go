// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/data"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkipSchemaCheck(t *testing.T) {
	initCmd := &cobra.Command{Use: "init"}
	buildCmd := &cobra.Command{Use: "build"}
	maandCmd.AddCommand(buildCmd)

	assert.True(t, skipSchemaCheck(initCmd))
	assert.True(t, skipSchemaCheck(maandCmd))
	assert.False(t, skipSchemaCheck(buildCmd))

	maandCmd.RemoveCommand(buildCmd)
}

func TestRequireCurrentSchemaBlocksV1Database(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	t.Cleanup(func() { bucket.Location = orig })

	require.NoError(t, os.MkdirAll(path.Join(root, "data"), 0o755))
	db, err := data.OpenDatabase(false)
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, data.MigrateSchema(tx))
	require.NoError(t, tx.Commit())
	require.NoError(t, db.Close())

	db, err = data.OpenDatabase(true)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE schema_version SET version = 1`)
	require.NoError(t, err)
	_, err = db.Exec(`DROP VIEW IF EXISTS cat_hashes`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	buildCmd := &cobra.Command{Use: "build"}
	maandCmd.AddCommand(buildCmd)
	t.Cleanup(func() { maandCmd.RemoveCommand(buildCmd) })

	err = requireCurrentSchema(buildCmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run maand init")
}
