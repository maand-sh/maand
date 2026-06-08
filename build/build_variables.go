// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/kv"
	"maand/utils"
	"maand/workspace"

	"github.com/pelletier/go-toml/v2"
)

func BuildVariables(tx *sql.Tx, jobWorkspace workspace.Workspace, removedWorkers, removedJobs []string) error {
	workerMeta, err := workerMetaByIP(jobWorkspace)
	if err != nil {
		return err
	}
	if err := buildWorkerVariables(tx, workerMeta, removedWorkers); err != nil {
		return err
	}
	if err := buildJobVariables(tx, removedJobs); err != nil {
		return err
	}
	if err := buildSharedWorkerVariables(tx); err != nil {
		return err
	}
	return buildBucketVariables(tx)
}

func workerMetaByIP(jobWorkspace workspace.Workspace) (map[string]workspace.Worker, error) {
	workers, err := jobWorkspace.ListWorkers()
	if err != nil {
		return nil, err
	}
	meta := make(map[string]workspace.Worker, len(workers))
	for _, worker := range workers {
		meta[worker.Host] = worker
	}
	return meta, nil
}

func buildWorkerVariables(tx *sql.Tx, workerMeta map[string]workspace.Worker, removedWorkers []string) error {
	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}

	for _, workerIP := range workers {
		variables := make(map[string]string)

		workerID, err := data.GetWorkerID(tx, workerIP)
		if err != nil {
			return err
		}

		variables["worker_ip"] = workerIP
		variables["worker_id"] = workerID

		workerLabels, err := data.GetWorkerLabels(tx, workerID)
		if err != nil {
			return err
		}
		variables["labels"] = strings.Join(workerLabels, ",")

		for _, label := range workerLabels {
			labelWorkers, err := data.GetWorkers(tx, []string{label})
			if err != nil {
				return err
			}
			peerWorkerIPs := utils.Difference(labelWorkers, []string{workerIP})
			if len(peerWorkerIPs) > 0 {
				variables[fmt.Sprintf("%s_peers", label)] = strings.Join(peerWorkerIPs, ",")
			}
		}

		labels, err := data.GetLabels(tx)
		if err != nil {
			return err
		}

		for _, label := range labels {
			labelWorkers, err := data.GetWorkers(tx, []string{label})
			if err != nil {
				return err
			}

			allocationIndex := -1
			for i, worker := range labelWorkers {
				if worker == workerIP {
					allocationIndex = i
					break
				}
			}
			if allocationIndex >= 0 {
				variables[fmt.Sprintf("%s_allocation_index", label)] = strconv.Itoa(allocationIndex)
			}
		}

		availableCPUMHz, err := data.GetWorkerCPU(tx, workerIP)
		if err != nil {
			return err
		}
		variables["worker_cpu_mhz"] = availableCPUMHz

		availableMemoryMB, err := data.GetWorkerMemory(tx, workerIP)
		if err != nil {
			return err
		}
		variables["worker_memory_mb"] = availableMemoryMB

		allocatedJobNames, err := data.GetActiveAllocatedJobs(tx, workerIP)
		if err != nil {
			return err
		}
		variables["jobs"] = strings.Join(allocatedJobNames, ",")

		position, err := data.GetWorkerPosition(tx, workerIP)
		if err != nil {
			return err
		}
		variables["position"] = strconv.Itoa(position)
		if worker, ok := workerMeta[workerIP]; ok && worker.Hostname != "" {
			variables["hostname"] = worker.Hostname
		}

		workerNamespace := fmt.Sprintf("maand/worker/%s", workerIP)
		err = syncKeyValues(tx, workerNamespace, variables)
		if err != nil {
			return err
		}

		tags, err := data.GetWorkerTags(tx, workerID)
		if err != nil {
			return err
		}
		tagsNamespace := fmt.Sprintf("maand/worker/%s/tags", workerIP)
		err = syncKeyValues(tx, tagsNamespace, tags)
		if err != nil {
			return err
		}
	}

	for _, workerIP := range removedWorkers {
		namespaces := []string{fmt.Sprintf("maand/worker/%s", workerIP)}
		allocatedJobNames, err := data.GetAllocatedJobs(tx, workerIP)
		if err != nil {
			return err
		}

		for _, jobName := range allocatedJobNames {
			namespaces = append(namespaces, fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIP))
		}

		for _, namespace := range namespaces {
			keys, err := kv.GetKVStore().GetKeys(namespace)
			if err != nil {
				return err
			}
			for _, key := range keys {
				err := kv.GetKVStore().Delete(namespace, key)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func buildSharedWorkerVariables(tx *sql.Tx) error {
	labels, err := data.GetLabels(tx)
	if err != nil {
		return err
	}

	sharedVariables := make(map[string]string)
	for _, label := range labels {
		labelWorkers, err := data.GetWorkers(tx, []string{label})
		if err != nil {
			return err
		}

		if len(labelWorkers) > 0 {
			sharedVariables[fmt.Sprintf("%s_label_id", label)] = workspace.GetHashUUID(label)
			sharedVariables[fmt.Sprintf("%s_workers", label)] = strings.Join(labelWorkers, ",")
			sharedVariables[fmt.Sprintf("%s_workers_length", label)] = strconv.Itoa(len(labelWorkers))
		}
		for idx, workerIP := range labelWorkers {
			sharedVariables[fmt.Sprintf("%s_%d", label, idx)] = workerIP
		}
	}

	caCertificate, err := os.ReadFile(path.Join(bucket.SecretLocation, "ca.crt"))
	if err != nil {
		return fmt.Errorf("%w: unable to read ca.crt", bucket.ErrUnexpectedError)
	}
	sharedVariables["certs/ca.crt"] = string(caCertificate)

	return syncKeyValues(tx, "maand/worker", sharedVariables)
}

func buildBucketVariables(tx *sql.Tx) error {
	bucketConfig := make(map[string]string)

	bucketConfPath := path.Join(bucket.WorkspaceLocation, "bucket.conf")

	if _, err := os.Stat(bucketConfPath); err == nil {
		bucketData, err := os.ReadFile(bucketConfPath)
		if err != nil {
			return fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
		}

		err = toml.Unmarshal(bucketData, &bucketConfig)
		if err != nil {
			return fmt.Errorf("%w: %w", bucket.ErrInvalidBucketConf, err)
		}
	}

	jobNames, err := data.GetJobs(tx)
	if err != nil {
		return err
	}

	maandVariables := make(map[string]string)
	for _, jobName := range jobNames {
		portMap, err := data.GetJobPortMap(tx, jobName)
		if err != nil {
			return err
		}
		for name, port := range portMap {
			maandVariables[name] = port
		}
	}
	maandVariables["jobs"] = strings.Join(jobNames, ",")
	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}
	maandVariables["bucket_id"] = bucketID

	err = syncKeyValues(tx, "vars/bucket", bucketConfig)
	if err != nil {
		return err
	}
	err = syncKeyValues(tx, "maand", maandVariables)
	if err != nil {
		return err
	}

	return nil
}

func loadJobBucketConfig(jobName string) (configFileName string, settings map[string]string, err error) {
	allJobSettings := make(map[string]map[string]string)

	maandConf, err := bucket.GetMaandConf()
	if err != nil {
		return "", nil, err
	}

	configFileName = "bucket.jobs.conf"
	maandConf.JobConfigSelector = strings.TrimSpace(maandConf.JobConfigSelector)
	if maandConf.JobConfigSelector != "" {
		configFileName = fmt.Sprintf("bucket.jobs.%s.conf", maandConf.JobConfigSelector)
	}
	configFilePath := path.Join(bucket.WorkspaceLocation, configFileName)

	if _, err := os.Stat(configFilePath); err == nil {
		configFileData, err := os.ReadFile(configFilePath)
		if err != nil {
			return "", nil, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
		}

		err = toml.Unmarshal(configFileData, &allJobSettings)
		if err != nil {
			return "", nil, fmt.Errorf("%w: bucket conf %s %w", bucket.ErrInvalidBucketConf, configFilePath, err)
		}
	}

	if _, ok := allJobSettings[jobName]; !ok {
		allJobSettings[jobName] = make(map[string]string)
	}

	return configFileName, allJobSettings[jobName], nil
}

func buildJobVariables(tx *sql.Tx, removedJobs []string) error {
	jobNames, err := data.GetJobs(tx)
	if err != nil {
		return err
	}

	for _, jobName := range jobNames {
		variables := make(map[string]string)

		variables["job_id"] = workspace.GetHashUUID(jobName)
		variables["name"] = jobName

		version, err := data.GetJobVersion(tx, jobName)
		if err != nil {
			return err
		}
		variables["version"] = data.NormalizeDeployVersion(version)

		jobSelectors, err := data.GetJobSelectors(tx, jobName)
		if err != nil {
			return err
		}
		variables["selectors"] = strings.Join(jobSelectors, ",")

		minMemory, maxMemory, err := data.GetJobMemoryLimits(tx, jobName)
		if err != nil {
			return err
		}
		variables["min_memory_mb"] = minMemory
		variables["max_memory_mb"] = maxMemory

		minCPU, maxCPU, err := data.GetJobCPULimits(tx, jobName)
		if err != nil {
			return err
		}
		variables["min_cpu_mhz"] = minCPU
		variables["max_cpu_mhz"] = maxCPU

		_, bucketJobSettings, err := loadJobBucketConfig(jobName)
		if err != nil {
			return err
		}

		variables["memory"], err = data.GetJobMemory(tx, jobName)
		if err != nil {
			return err
		}
		if minMemory == "0" && maxMemory == "0" {
			variables["min_memory_mb"] = variables["memory"]
			variables["max_memory_mb"] = variables["memory"]
		}

		variables["cpu"], err = data.GetJobCPU(tx, jobName)
		if err != nil {
			return err
		}
		if minCPU == "0" && maxCPU == "0" {
			variables["min_cpu_mhz"] = variables["cpu"]
			variables["max_cpu_mhz"] = variables["cpu"]
		}

		activeWorkers, err := data.GetActiveAllocationsOrdered(tx, jobName)
		if err != nil {
			return err
		}
		variables["workers"] = strings.Join(activeWorkers, ",")
		variables["workers_length"] = strconv.Itoa(len(activeWorkers))
		for idx, workerIP := range activeWorkers {
			variables[fmt.Sprintf("worker_%d", idx)] = workerIP
		}

		portMap, err := data.GetJobPortMap(tx, jobName)
		if err != nil {
			return err
		}
		if len(portMap) > 0 {
			portNames := make([]string, 0, len(portMap))
			for name := range portMap {
				portNames = append(portNames, name)
			}
			sort.Strings(portNames)
			for _, name := range portNames {
				variables[name] = portMap[name]
			}
			portsJSON, err := json.Marshal(portMap)
			if err != nil {
				return fmt.Errorf("%w: marshal ports_json for job %s: %w", bucket.ErrUnexpectedError, jobName, err)
			}
			variables["ports_json"] = string(portsJSON)
		}

		jobNamespace := fmt.Sprintf("maand/job/%s", jobName)
		err = syncKeyValues(tx, jobNamespace, variables)
		if err != nil {
			return err
		}

		bucketJobNamespace := fmt.Sprintf("vars/bucket/job/%s", jobName)
		err = syncKeyValues(tx, bucketJobNamespace, bucketJobSettings)
		if err != nil {
			return err
		}

		if err := mergeWorkspaceJobVars(jobName); err != nil {
			return err
		}
	}

	for _, jobName := range removedJobs {
		namespaces := []string{
			fmt.Sprintf("maand/job/%s", jobName),
			fmt.Sprintf("vars/job/%s", jobName),
			fmt.Sprintf("vars/bucket/job/%s", jobName),
		}
		allocatedWorkerIPs, err := data.GetAllocatedWorkers(tx, jobName)
		if err != nil {
			return err
		}

		for _, workerIP := range allocatedWorkerIPs {
			namespaces = append(namespaces, fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIP))
		}

		for _, namespace := range namespaces {
			err := syncKeyValues(tx, namespace, make(map[string]string))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// mergeWorkspaceJobVars applies jobs/<job>/vars.toml into vars/job/<job> without deleting
// other keys (user/script-owned config survives rebuild).
func mergeWorkspaceJobVars(jobName string) error {
	varsPath := path.Join(bucket.WorkspaceLocation, "jobs", jobName, "vars.toml")
	data, err := os.ReadFile(varsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%w: read %s: %w", bucket.ErrUnexpectedError, varsPath, err)
	}

	vars := make(map[string]string)
	if err := toml.Unmarshal(data, &vars); err != nil {
		return fmt.Errorf("%w: %s: %w", bucket.ErrInvalidJobVars, varsPath, err)
	}

	namespace := fmt.Sprintf("vars/job/%s", jobName)
	for key, value := range vars {
		kv.GetKVStore().Put(namespace, key, value, 0)
	}
	return nil
}

func syncKeyValues(tx *sql.Tx, namespace string, keyValues map[string]string) error {
	var presentKeys []string
	for key, value := range keyValues {
		kv.GetKVStore().Put(namespace, key, value, 0)
		presentKeys = append(presentKeys, key)
	}

	existingKeys, err := kv.GetKVStore().GetKeys(namespace)
	if err != nil {
		return err
	}

	staleKeys := utils.Difference(existingKeys, presentKeys)
	for _, staleKey := range staleKeys {
		err := kv.GetKVStore().Delete(namespace, staleKey)
		if err != nil {
			return err
		}
	}

	return nil
}

// bucket.conf => vars/bucket
// workers meta => maand/worker, maand/worker/10.0.0.1
// worker tags => maand/worker/10.0.0.1/tags
// custom job variables = vars/job/a
// bucket.jobs.conf => vars/bucket/job/a
// bucket.jobs.conf (memory, cpu) => maand/job/a
// job resources (memory and cpu) => maand/job/a
// job resources (ports) => maand
// job meta => maand/job/a
// job certs => maand/job/a/worker/10.0.0.1
