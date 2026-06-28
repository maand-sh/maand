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
	workers, err := data.GetNonRemovedAllocations(tx, job)
	if err != nil {
		return err
	}

	namespace := fmt.Sprintf("%s_allocation", job)

	for _, workerIP := range workers {
		workerDirPath := bucket.GetTempWorkerPath(workerIP)
		jobDir := path.Join(workerDirPath, "jobs", job)

		allocID, err := data.GetAllocationID(tx, workerIP, job)
		if err != nil {
			return err
		}

		tree, err := utils.HashDirectoryTree(jobDir)
		if err != nil {
			return err
		}
		if err := data.UpdateAllocationPlan(tx, namespace, allocID, tree.Aggregate, data.FileManifest(tree.Files)); err != nil {
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
	workers, err := data.GetNonRemovedAllocations(tx, job)
	if err != nil {
		return err
	}

	namespace := fmt.Sprintf("%s_allocation", job)
	for _, workerIP := range workers {
		allocID, err := data.GetAllocationID(tx, workerIP, job)
		if err != nil {
			return err
		}

		if err := data.PromoteAllocationState(tx, namespace, allocID); err != nil {
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
