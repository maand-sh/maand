// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package worker

import (
	"fmt"
	"log"

	"maand/bucket"
)

// JobDeployArtifactsCleanupCommand returns the remote shell command that removes
// deployed job files while preserving data/ and logs/ for later redeploy or GC.
func JobDeployArtifactsCleanupCommand(bucketID, job string) string {
	base := fmt.Sprintf("/opt/worker/%s/jobs/%s", bucketID, job)
	return fmt.Sprintf(
		`if [ -d %q ]; then find %q -mindepth 1 -maxdepth 1 ! -name data ! -name logs -exec rm -rf {} +; fi`,
		base, base,
	)
}

// RemoveJobDeployArtifacts deletes deployed job files on a worker but leaves
// data/ and logs/ in place so the same allocation can reuse them on redeploy.
// Permanent deletion of data/, logs/, and bin/ is handled by maand gc.
func RemoveJobDeployArtifacts(rt *bucket.Runtime, bucketID, workerIP, job string) error {
	cmd := JobDeployArtifactsCleanupCommand(bucketID, job)
	if err := ExecuteCommand(rt, workerIP, []string{cmd}, nil); err != nil {
		return remoteError(workerIP, err)
	}
	return nil
}

// RemoveJobRuntimeDirs deletes data/, logs/, and bin/ for a job on a worker.
func RemoveJobRuntimeDirs(rt *bucket.Runtime, bucketID, workerIP, job string) error {
	base := fmt.Sprintf("/opt/worker/%s/jobs/%s", bucketID, job)
	cmd := fmt.Sprintf("rm -rf %s/data %s/logs %s/bin", base, base, base)
	if err := ExecuteCommand(rt, workerIP, []string{cmd}, nil); err != nil {
		return remoteError(workerIP, err)
	}
	return nil
}

// RemoveJobTree deletes the entire jobs/<job> directory on a worker (maand gc).
func RemoveJobTree(rt *bucket.Runtime, bucketID, workerIP, job string) error {
	base := fmt.Sprintf("/opt/worker/%s/jobs/%s", bucketID, job)
	cmd := fmt.Sprintf("rm -rf %q", base)
	if err := ExecuteCommand(rt, workerIP, []string{cmd}, nil); err != nil {
		return remoteError(workerIP, err)
	}
	return nil
}

// RemoveJobTreeOrAssumeDead is like RemoveJobTree but ignores SSH errors when the
// worker was removed from the catalog and is assumed dead.
func RemoveJobTreeOrAssumeDead(rt *bucket.Runtime, bucketID, workerIP, job string) {
	if err := RemoveJobTree(rt, bucketID, workerIP, job); err != nil {
		log.Printf("gc: removed worker %s unreachable, assuming dead: %v", workerIP, err)
	}
}
