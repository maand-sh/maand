// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func TestCloseRowsNil(t *testing.T) {
	require.NoError(t, closeRows(nil))
}

func TestRowsErrNil(t *testing.T) {
	require.NoError(t, RowsErr(nil))
}

func TestCloseRowsAfterQuery(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`SELECT 1`)
	require.NoError(t, err)
	require.NoError(t, closeRows(rows))
}

func TestRowsErrAfterIteration(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`SELECT 1`)
	require.NoError(t, err)
	require.True(t, rows.Next())
	assert.NoError(t, RowsErr(rows))
	require.NoError(t, rows.Close())
}
