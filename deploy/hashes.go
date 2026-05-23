// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"
	"path"

	"maand/bucket"
	"maand/data"
	"maand/utils"
)

func updateAllocationHash(tx *sql.Tx, jobs []string) error {
	for _, job := range jobs {
		if err := updateJobAllocationHashes(tx, job); err != nil {
			return err
		}
	}
	return nil
}

func updateJobAllocationHashes(tx *sql.Tx, job string) error {
	targetVersion, err := data.TargetJobVersion(tx, job)
	if err != nil {
		return err
	}

	allocatedWorkers, err := data.GetAllocatedWorkers(tx, job)
	if err != nil {
		return err
	}

	namespace := fmt.Sprintf("%s_allocation", job)

	for _, workerIP := range allocatedWorkers {
		removed, err := data.IsAllocationRemoved(tx, workerIP, job)
		if err != nil {
			return err
		}
		if removed == 1 {
			allocID, err := data.GetAllocationID(tx, workerIP, job)
			if err != nil {
				return err
			}
			if err := data.RemoveHash(tx, namespace, allocID); err != nil {
				return err
			}
			continue
		}

		disabled, err := data.IsAllocationDisabled(tx, workerIP, job)
		if err != nil {
			return err
		}
		if disabled == 1 {
			continue
		}

		workerDirPath := bucket.GetTempWorkerPath(workerIP)
		jobDir := path.Join(workerDirPath, "jobs", job)

		allocID, err := data.GetAllocationID(tx, workerIP, job)
		if err != nil {
			return err
		}

		md5, err := utils.HashDirectory(jobDir)
		if err != nil {
			return err
		}
		if err := data.UpdateAllocationPlan(tx, namespace, allocID, md5, targetVersion); err != nil {
			return err
		}

		versions, err := data.GetAllocationVersions(tx, namespace, allocID)
		if err != nil {
			return err
		}
		if err := syncAllocationVersionKV(job, workerIP, versions); err != nil {
			return err
		}
	}
	return nil
}

func promoteAllocationHash(tx *sql.Tx, job string) error {
	allocatedWorkers, err := data.GetAllocatedWorkers(tx, job)
	if err != nil {
		return err
	}

	namespace := fmt.Sprintf("%s_allocation", job)
	for _, workerIP := range allocatedWorkers {
		allocID, err := data.GetAllocationID(tx, workerIP, job)
		if err != nil {
			return err
		}

		disabled, err := data.IsAllocationDisabled(tx, workerIP, job)
		if err != nil {
			return err
		}
		if disabled == 1 {
			if err := data.ClearAllocationLiveState(tx, namespace, allocID); err != nil {
				return err
			}
			continue
		}
		if err := data.PromoteAllocationState(tx, namespace, allocID); err != nil {
			return err
		}
	}
	return nil
}
