// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerCatalogContainsNil(t *testing.T) {
	var catalog WorkerCatalog
	assert.False(t, catalog.Contains("10.0.0.1"))
}

func TestNewWorkerCatalogAndContains(t *testing.T) {
	catalog := NewWorkerCatalog([]string{"10.0.0.1", "10.0.0.2"})
	assert.True(t, catalog.Contains("10.0.0.1"))
	assert.False(t, catalog.Contains("10.0.0.3"))
}

func TestLoadWorkerCatalog(t *testing.T) {
	db := openMigratedTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	seedWorkerJobAllocation(t, tx)
	require.NoError(t, tx.Commit())

	tx, err = db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	catalog, err := LoadWorkerCatalog(tx)
	require.NoError(t, err)
	assert.True(t, catalog.Contains("10.0.0.1"))
	assert.True(t, catalog.Contains("10.0.0.2"))
}
