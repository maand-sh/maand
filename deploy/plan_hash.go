// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"os"

	"maand/bucket"
)

// refreshPlanHashesForJobs stages jobs under tmp/workers/ and updates <job>_allocation
// current_hash from the rendered tree. Run at deploy time (before JobNeedsRollout) so
// content changes since the last promote are visible without build knowing about hashes.
func refreshPlanHashesForJobs(tx *sql.Tx, jobs []string) error {
	if len(jobs) == 0 {
		return nil
	}

	if err := os.MkdirAll(bucket.TempLocation, 0o755); err != nil {
		return bucket.UnexpectedError(err)
	}

	if err := prepareJobsFiles(tx, jobs); err != nil {
		return err
	}
	return updateAllocationHash(tx, jobs)
}
