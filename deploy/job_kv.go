// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"

	"maand/data"
	"maand/kv"
)

func purgeJobCommandKVForInactiveJobs(tx *sql.Tx, stopped []data.StoppedAllocation) error {
	if len(stopped) == 0 {
		return nil
	}

	store, err := kv.RequireStore()
	if err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(stopped))
	for _, alloc := range stopped {
		if _, ok := seen[alloc.Job]; ok {
			continue
		}
		seen[alloc.Job] = struct{}{}

		retained, err := data.JobHasNonRemovedAllocations(tx, alloc.Job)
		if err != nil {
			return err
		}
		if retained {
			continue
		}

		for _, namespace := range data.JobKVNamespaces(alloc.Job) {
			store.PurgeNamespace(namespace)
		}
		workerIPs, err := data.GetAllocatedWorkers(tx, alloc.Job)
		if err != nil {
			return err
		}
		for _, workerIP := range workerIPs {
			store.PurgeNamespace(fmt.Sprintf("maand/job/%s/worker/%s", alloc.Job, workerIP))
		}
	}

	return kv.PersistToTransaction(tx, store)
}
