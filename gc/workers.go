// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gc

import (
	"database/sql"

	"maand/bucket"
	"maand/data"
	"maand/worker"
)

func purgeRemovedAllocationWorkerData(
	rt *bucket.Runtime,
	tx *sql.Tx,
	bucketID string,
	allocs []removedAllocation,
) error {
	catalog, err := data.LoadWorkerCatalog(tx)
	if err != nil {
		return err
	}

	for _, alloc := range allocs {
		if catalog.Contains(alloc.WorkerIP) {
			if err := worker.RemoveJobTree(rt, bucketID, alloc.WorkerIP, alloc.Job); err != nil {
				return err
			}
			continue
		}
		worker.RemoveJobTreeOrAssumeDead(rt, bucketID, alloc.WorkerIP, alloc.Job)
	}
	return nil
}
