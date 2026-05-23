// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package worker

import (
	"fmt"
	"log"

	"maand/bucket"
)

// RemoveJobRuntimeDirs deletes data/, logs/, and bin/ for a job on a worker.
func RemoveJobRuntimeDirs(rt *bucket.Runtime, bucketID, workerIP, job string) error {
	base := fmt.Sprintf("/opt/worker/%s/jobs/%s", bucketID, job)
	cmd := fmt.Sprintf("rm -rf %s/data %s/logs %s/bin", base, base, base)
	if err := ExecuteCommand(rt, workerIP, []string{cmd}, nil); err != nil {
		return remoteError(workerIP, err)
	}
	return nil
}

// RemoveJobRuntimeDirsOrAssumeDead is like RemoveJobRuntimeDirs but ignores SSH errors
// when the worker was removed from the catalog and is assumed dead.
func RemoveJobRuntimeDirsOrAssumeDead(rt *bucket.Runtime, bucketID, workerIP, job string) {
	if err := RemoveJobRuntimeDirs(rt, bucketID, workerIP, job); err != nil {
		log.Printf("gc: removed worker %s unreachable, assuming dead: %v", workerIP, err)
	}
}
