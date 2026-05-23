// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"

	"maand/kv"
)

// persistJobCommandKV flushes in-memory KV changes from job commands into the deploy transaction.
// Called after each job's pre_deploy and post_deploy hooks; changes commit with the deploy tx.
func persistJobCommandKV(tx *sql.Tx, job string) error {
	if err := kv.PersistToSessionTransaction(tx); err != nil {
		return &JobError{Job: job, Err: fmt.Errorf("persist job command kv: %w", err)}
	}
	return nil
}
