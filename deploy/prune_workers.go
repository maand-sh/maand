// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"fmt"
	"os"
	"path"

	"maand/bucket"
)

func removeJobDirectoryFromWorker(
	rt *bucket.Runtime,
	bucketID, workerIP, job string,
	assumeDead bool,
) error {
	remoteDir := fmt.Sprintf("/opt/worker/%s/jobs/%s", bucketID, job)
	err := runWorkerCommand(
		rt,
		workerIP,
		[]string{fmt.Sprintf("rm -rf %s", remoteDir)},
		nil,
	)
	if err = finishRemovedWorkerCommand(workerIP, err, assumeDead); err != nil {
		return fmt.Errorf("worker %s job %s: %w", workerIP, job, err)
	}
	localDir := path.Join(bucket.GetTempWorkerPath(workerIP), "jobs", job)
	if err := os.RemoveAll(localDir); err != nil && !os.IsNotExist(err) {
		return bucket.UnexpectedError(err)
	}
	return nil
}

func removeWorkerBucketFromWorker(rt *bucket.Runtime, bucketID, workerIP string) error {
	remoteDir := fmt.Sprintf("/opt/worker/%s", bucketID)
	runWorkerCommandOrAssumeDead(
		rt,
		workerIP,
		[]string{fmt.Sprintf("rm -rf %s", remoteDir)},
		nil,
	)
	localDir := bucket.GetTempWorkerPath(workerIP)
	if err := os.RemoveAll(localDir); err != nil && !os.IsNotExist(err) {
		return bucket.UnexpectedError(err)
	}
	return nil
}
