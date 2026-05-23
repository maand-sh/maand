// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"testing"

	"maand/data"
	"maand/gc"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPersistsKVChanges(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	runBuild(t)

	value, err := GetKey("maand/worker/10.0.0.1", "worker_ip")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", value)
}

func TestGCRequiresInitializedBucket(t *testing.T) {
	resetBucket(t)
	err := gc.Execute(0)
	assert.Error(t, err)
}

func TestGCRoundTripAfterBuild(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	runBuild(t)

	before := MustQueryCount(t, `SELECT count(*) FROM key_value`)
	require.NoError(t, gc.Execute(0))
	after := MustQueryCount(t, `SELECT count(*) FROM key_value`)
	assert.LessOrEqual(t, after, before)
}

func TestKVSessionReloadAfterGC(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	runBuild(t)

	require.NoError(t, gc.Execute(0))

	// Re-init session like a follow-up command would.
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

	require.NoError(t, kv.Initialize(tx))
	store, err := kv.RequireStore()
	require.NoError(t, err)
	assert.NotNil(t, store)
}
