// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gc

import (
	"database/sql"
	"fmt"

	"maand/data"
	"maand/kv"
)

func purgeRemovedAllocationReferences(
	tx *sql.Tx,
	store *kv.Store,
	allocs []removedAllocation,
) error {
	if len(allocs) == 0 {
		return nil
	}

	catalog, err := data.LoadWorkerCatalog(tx)
	if err != nil {
		return err
	}

	offCatalogWorkers := make(map[string]struct{})
	for _, alloc := range allocs {
		store.PurgeNamespace(fmt.Sprintf("maand/job/%s/worker/%s", alloc.Job, alloc.WorkerIP))
		if !catalog.Contains(alloc.WorkerIP) {
			offCatalogWorkers[alloc.WorkerIP] = struct{}{}
		}
	}

	for workerIP := range offCatalogWorkers {
		store.PurgeNamespace(fmt.Sprintf("maand/worker/%s", workerIP))
		store.PurgeNamespace(fmt.Sprintf("maand/worker/%s/tags", workerIP))
	}

	if err := purgeRemovedJobNamespaces(tx, store, allocs); err != nil {
		return err
	}

	return kv.PersistToTransaction(tx, store)
}

func purgeRemovedJobNamespaces(tx *sql.Tx, store *kv.Store, allocs []removedAllocation) error {
	seen := make(map[string]struct{}, len(allocs))
	for _, alloc := range allocs {
		if _, ok := seen[alloc.Job]; ok {
			continue
		}
		seen[alloc.Job] = struct{}{}

		active, err := data.JobHasActiveAllocations(tx, alloc.Job)
		if err != nil {
			return err
		}
		if active {
			continue
		}

		for _, namespace := range data.JobKVNamespaces(alloc.Job) {
			store.PurgeNamespace(namespace)
		}
	}
	return nil
}
