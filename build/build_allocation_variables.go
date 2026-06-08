// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"fmt"
	"sort"
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
	activeWorkers, err := data.GetActiveAllocationsOrdered(tx, jobName)
	if err != nil {
		return err
	}

	portMap, err := data.GetJobPortMap(tx, jobName)
	if err != nil {
		return err
	}

	indexKey := jobName + "_allocation_index"
	for idx, workerIP := range activeWorkers {
		peers := make([]string, 0, len(activeWorkers)-1)
		for _, peer := range activeWorkers {
			if peer != workerIP {
				peers = append(peers, peer)
			}
		}

		vars := map[string]string{
			indexKey:       strconv.Itoa(idx),
			"is_primary":   boolString(idx == 0),
			"is_seed":      boolString(idx == 0),
			"peer_workers": strings.Join(peers, ","),
			"peer_ports":   encodePeerPorts(peers, portMap),
		}

		namespace := fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIP)
		if err := syncAllocationKeyValues(namespace, vars); err != nil {
			return err
		}
	}
	return purgeStaleJobAllocationVariables(tx, jobName, activeWorkers)
}

func boolString(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func encodePeerPorts(peerIPs []string, portMap map[string]string) string {
	if len(peerIPs) == 0 || len(portMap) == 0 {
		return ""
	}
	names := make([]string, 0, len(portMap))
	for name := range portMap {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(peerIPs)*len(names))
	for _, peer := range peerIPs {
		for _, name := range names {
			parts = append(parts, peer+":"+name+":"+portMap[name])
		}
	}
	return strings.Join(parts, ",")
}

func purgeStaleJobAllocationVariables(tx *sql.Tx, jobName string, activeWorkers []string) error {
	allocatedWorkers, err := data.GetAllocatedWorkers(tx, jobName)
	if err != nil {
		return err
	}
	active := make(map[string]struct{}, len(activeWorkers))
	for _, workerIP := range activeWorkers {
		active[workerIP] = struct{}{}
	}
	for _, workerIP := range allocatedWorkers {
		if _, ok := active[workerIP]; ok {
			continue
		}
		namespace := fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIP)
		if err := syncAllocationKeyValues(namespace, map[string]string{}); err != nil {
			return err
		}
	}
	return nil
}

// syncAllocationKeyValues updates allocation metadata without removing cert keys.
func syncAllocationKeyValues(namespace string, keyValues map[string]string) error {
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
		if strings.HasPrefix(key, "certs/") {
			continue
		}
		if key == "version" {
			continue
		}
		if err := kv.GetKVStore().Delete(namespace, key); err != nil {
			return err
		}
	}
	return nil
}
