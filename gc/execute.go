// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package gc removes stale database rows after builds and manual cleanup.
package gc

import (
	"maand/bucket"
	"maand/data"
	"maand/kv"
)

const defaultKVRetainDays = 0

// Execute purges removed allocations and stale key_value history.
// retainDays controls how long deleted KV rows are kept (0 = purge eligible rows immediately).
func Execute(retainDays int) error {
	db, err := data.OpenDatabase(true)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := kv.Initialize(tx); err != nil {
		return err
	}

	removedAllocs, err := listRemovedAllocations(tx)
	if err != nil {
		return err
	}

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

	rt, err := bucket.SetupRuntime(bucketID)
	if err != nil {
		return err
	}
	defer func() {
		_ = rt.Stop()
	}()

	if len(removedAllocs) > 0 {
		if err := purgeRemovedAllocationWorkerData(rt, tx, bucketID, removedAllocs); err != nil {
			return err
		}
	}

	store, err := kv.RequireStore()
	if err != nil {
		return err
	}

	if len(removedAllocs) > 0 {
		if err := purgeRemovedAllocationReferences(tx, store, removedAllocs); err != nil {
			return err
		}
	}

	if err := purgeRemovedAllocations(tx); err != nil {
		return err
	}

	if err := store.PurgeStaleVersions(tx, retainDays); err != nil {
		return err
	}

	return tx.Commit()
}

// Collect runs GC with default retention (purge stale KV rows not kept by version window).
func Collect() error {
	return Execute(defaultKVRetainDays)
}
