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

func allocationVersionEnv(tx *sql.Tx, job, workerIP string) ([]string, error) {
	allocID, err := data.GetAllocationID(tx, workerIP, job)
	if err != nil {
		return nil, err
	}
	namespace := fmt.Sprintf("%s_allocation", job)
	versions, err := data.GetAllocationVersions(tx, namespace, allocID)
	if err != nil {
		return nil, err
	}
	return []string{
		fmt.Sprintf("CURRENT_VERSION=%s", versions.CurrentVersion),
		fmt.Sprintf("NEW_VERSION=%s", versions.NewVersion),
	}, nil
}

func syncAllocationVersionKV(job, workerIP string, versions data.AllocationVersions) error {
	store := kv.GetKVStore()
	if store == nil {
		return nil
	}
	namespace := fmt.Sprintf("maand/job/%s/worker/%s", job, workerIP)
	store.Put(namespace, "current_version", versions.CurrentVersion, 0)
	store.Put(namespace, "new_version", versions.NewVersion, 0)
	return nil
}

func allocationVersionsForWorker(tx *sql.Tx, job, workerIP string) (data.AllocationVersions, error) {
	allocID, err := data.GetAllocationID(tx, workerIP, job)
	if err != nil {
		return data.AllocationVersions{}, err
	}
	namespace := fmt.Sprintf("%s_allocation", job)
	return data.GetAllocationVersions(tx, namespace, allocID)
}
