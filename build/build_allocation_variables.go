// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"maand/data"
	"maand/kv"
	"maand/utils"
)

// BuildJobAllocationVariables writes per-allocation KV after certs are synced.
// Call after BuildCerts so cert sync does not delete these keys.
func BuildJobAllocationVariables(tx *sql.Tx, removedJobs []string) error {
	jobNames, err := data.GetJobs(tx)
	if err != nil {
		return err
	}

	for _, jobName := range jobNames {
		if err := syncJobAllocationVariables(tx, jobName); err != nil {
			return err
		}
	}

	for _, jobName := range removedJobs {
		allocatedWorkerIPs, err := data.GetAllocatedWorkers(tx, jobName)
		if err != nil {
			return err
		}
		for _, workerIP := range allocatedWorkerIPs {
			namespace := fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIP)
			if err := syncKeyValues(tx, namespace, map[string]string{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func syncJobAllocationVariables(tx *sql.Tx, jobName string) error {
	workers, err := data.GetNonRemovedAllocationsOrdered(tx, jobName)
	if err != nil {
		return err
	}

	indexKey := jobName + "_allocation_index"
	for idx, workerIP := range workers {
		peers := make([]string, 0, len(workers)-1)
		for _, peer := range workers {
			if peer != workerIP {
				peers = append(peers, peer)
			}
		}

		vars := map[string]string{
			indexKey:       strconv.Itoa(idx),
			"peer_workers": strings.Join(peers, ","),
		}

		namespace := fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIP)
		if err := syncAllocationKeyValues(namespace, vars); err != nil {
			return err
		}
	}
	return purgeStaleJobAllocationVariables(tx, jobName)
}

func purgeStaleJobAllocationVariables(tx *sql.Tx, jobName string) error {
	nonRemovedWorkers, err := data.GetNonRemovedAllocations(tx, jobName)
	if err != nil {
		return err
	}
	retained := make(map[string]struct{}, len(nonRemovedWorkers))
	for _, workerIP := range nonRemovedWorkers {
		retained[workerIP] = struct{}{}
	}

	allocatedWorkers, err := data.GetAllocatedWorkers(tx, jobName)
	if err != nil {
		return err
	}
	for _, workerIP := range allocatedWorkers {
		if _, ok := retained[workerIP]; ok {
			continue
		}
		namespace := fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIP)
		if err := syncKeyValues(tx, namespace, map[string]string{}); err != nil {
			return err
		}
	}
	return nil
}

// syncAllocationKeyValues updates allocation metadata without removing cert keys.
func syncAllocationKeyValues(namespace string, keyValues map[string]string) error {
	return syncNamespaceKeyValues(namespace, keyValues, syncKeyPolicyAllocation)
}

// syncCertKeyValues updates cert PEMs without removing allocation metadata keys.
func syncCertKeyValues(namespace string, keyValues map[string]string) error {
	return syncNamespaceKeyValues(namespace, keyValues, syncKeyPolicyCert)
}

type syncKeyPolicy int

const (
	syncKeyPolicyFull syncKeyPolicy = iota
	syncKeyPolicyAllocation
	syncKeyPolicyCert
)

func syncNamespaceKeyValues(namespace string, keyValues map[string]string, policy syncKeyPolicy) error {
	presentKeys := make([]string, 0, len(keyValues))
	for key, value := range keyValues {
		kv.GetKVStore().Put(namespace, key, value, 0)
		presentKeys = append(presentKeys, key)
	}

	existingKeys, err := kv.GetKVStore().GetKeys(namespace)
	if err != nil {
		return err
	}

	staleKeys := utils.Difference(existingKeys, presentKeys)
	for _, key := range staleKeys {
		switch policy {
		case syncKeyPolicyAllocation:
			if strings.HasPrefix(key, "certs/") {
				continue
			}
			if key == "version" {
				continue
			}
		case syncKeyPolicyCert:
			if !strings.HasPrefix(key, "certs/") {
				continue
			}
		}
		if err := kv.GetKVStore().Delete(namespace, key); err != nil {
			return err
		}
	}
	return nil
}
