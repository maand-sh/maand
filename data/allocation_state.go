// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import "database/sql"

// IsActiveAllocation reports whether an allocation may be started on deploy
// (removed=0 and disabled=0). Disabled allocations remain in the catalog and
// receive build KV, certs, and deploy staging; only start/restart/rsync are skipped.
func IsActiveAllocation(removed, disabled int) bool {
	return removed == 0 && disabled == 0
}

// IsAllocationActive loads allocation flags and reports whether the row is active.
func IsAllocationActive(tx *sql.Tx, workerIP, job string) (bool, error) {
	removed, err := IsAllocationRemoved(tx, workerIP, job)
	if err != nil {
		return false, err
	}
	if removed == 1 {
		return false, nil
	}
	disabled, err := IsAllocationDisabled(tx, workerIP, job)
	if err != nil {
		return false, err
	}
	return disabled == 0, nil
}

// StoppedAllocationAssumeDead reports whether a removed allocation's worker is off-catalog
// and SSH cleanup should best-effort when unreachable.
func StoppedAllocationAssumeDead(alloc StoppedAllocation, catalog WorkerCatalog) bool {
	return alloc.Removed && !catalog.Contains(alloc.WorkerIP)
}
