// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/kv"
	"maand/utils"
	"maand/workspace"

	"github.com/pelletier/go-toml/v2"
)

func Variables(tx *sql.Tx) error {
	err := processWorkerData(tx)
	if err != nil {
		return err
	}
	err = processJobData(tx)
	if err != nil {
		return err
	}
	err = processLabelData(tx)
	if err != nil {
		return err
	}
	err = processBucketConf(tx)
	if err != nil {
		return err
	}
	return nil
}

func processWorkerData(tx *sql.Tx) error {
	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}

	for _, workerIP := range workers {
		workerKV := make(map[string]string)

		workerID, err := data.GetWorkerID(tx, workerIP)
		if err != nil {
			return err
		}

		workerKV["worker_ip"] = workerIP
		workerKV["worker_id"] = workerID

		workerLabels, err := data.GetWorkerLabels(tx, workerID)
		if err != nil {
			return err
		}
		workerKV["labels"] = strings.Join(workerLabels, ",")

		for _, label := range workerLabels {
			labelWorkers, err := data.GetWorkers(tx, []string{label})
			if err != nil {
				return err
			}
			peers := utils.Difference(labelWorkers, []string{workerIP})
			if len(peers) > 0 {
				workerKV[fmt.Sprintf("%s_peers", label)] = strings.Join(peers, ",")
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

			index := -1 // Default value if workerIP is not found
			for i, worker := range labelWorkers {
				if worker == workerIP {
					index = i
					break
				}
			}
			if index >= 0 {
				workerKV[fmt.Sprintf("%s_allocation_index", label)] = strconv.Itoa(index)
			}
		}

		availableCPUMhz, err := data.GetWorkerCPU(tx, workerIP)
		if err != nil {
			return err
		}
		workerKV["worker_cpu_mhz"] = availableCPUMhz

		availableMemoryMb, err := data.GetWorkerMemory(tx, workerIP)
		if err != nil {
			return err
		}
		workerKV["worker_memory_mb"] = availableMemoryMb

		allocatedJobs, err := data.GetAllocatedJobs(tx, workerIP)
		if err != nil {
			return err
		}
		workerKV["jobs"] = strings.Join(allocatedJobs, ",")

		workerKVNamespace := fmt.Sprintf("maand/worker/%s", workerIP)
		err = storeKeyValues(tx, workerKVNamespace, workerKV)
		if err != nil {
			return err
		}

		tags, err := data.GetWorkerTags(tx, workerID)
		if err != nil {
			return err
		}
		tagsNS := fmt.Sprintf("maand/worker/%s/tags", workerIP)
		err = storeKeyValues(tx, tagsNS, tags)
		if err != nil {
			return err
		}
	}

	for _, workerIP := range removedWorkers {
		nss := []string{fmt.Sprintf("maand/worker/%s", workerIP)}
		workerJobs, err := data.GetAllocatedJobs(tx, workerIP)
		if err != nil {
			return err
		}

		for _, job := range workerJobs {
			nss = append(nss, fmt.Sprintf("maand/job/%s/worker/%s", job, workerIP))
		}

		for _, ns := range nss {
			keys, _ := kv.GetKVStore().GetKeys(tx, ns)
			for _, key := range keys {
				err := kv.GetKVStore().Delete(tx, ns, key)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func processLabelData(tx *sql.Tx) error {
	labels, err := data.GetLabels(tx)
	if err != nil {
		return err
	}

	workerKV := make(map[string]string)
	for _, label := range labels {
		labelWorkers, err := data.GetWorkers(tx, []string{label})
		if err != nil {
			return err
		}

		if len(labelWorkers) > 0 {
			workerKV[fmt.Sprintf("%s_label_id", label)] = workspace.GetHashUUID(label)
			workerKV[fmt.Sprintf("%s_workers", label)] = strings.Join(labelWorkers, ",")
			workerKV[fmt.Sprintf("%s_workers_length", label)] = strconv.Itoa(len(labelWorkers))
		}
		for idx, node := range labelWorkers {
			workerKV[fmt.Sprintf("%s_%d", label, idx)] = node
		}
	}

	content, err := os.ReadFile(path.Join(bucket.SecretLocation, "ca.crt"))
	if err != nil {
		return fmt.Errorf("%w: unable to read ca.crt", bucket.ErrUnexpectedError)
	}
	workerKV["certs/ca.crt"] = string(content)

	return storeKeyValues(tx, "maand/worker", workerKV)
}

func processBucketConf(tx *sql.Tx) error {
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

	availableJobs, err := data.GetJobs(tx)
	if err != nil {
		return err
	}

	jobPorts := make(map[string]string)
	for _, job := range availableJobs {
		rows, err := tx.Query("SELECT name, port FROM job_ports WHERE job_id = (SELECT job_id FROM job WHERE name = ?)", job)
		if err != nil {
			return bucket.DatabaseError(err)
		}

		for rows.Next() {
			var name, value string
			err := rows.Scan(&name, &value)
			if err != nil {
				return bucket.DatabaseError(err)
			}
			jobPorts[name] = value
		}
	}

	err = storeKeyValues(tx, "vars/bucket", bucketConfig)
	if err != nil {
		return err
	}
	err = storeKeyValues(tx, "maand", jobPorts)
	if err != nil {
		return err
	}

	return nil
}

func getJobConf(job string) (map[string]string, error) {
	config := make(map[string]map[string]string)

	maandConf, err := bucket.GetMaandConf()
	if err != nil {
		return nil, err
	}

	bucketJobConf := path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf")
	maandConf.JobConfigSelector = strings.TrimSpace(maandConf.JobConfigSelector)
	if maandConf.JobConfigSelector != "" {
		bucketJobConf = path.Join(bucket.WorkspaceLocation, fmt.Sprintf("bucket.jobs.%s.conf", maandConf.JobConfigSelector))
	}

	if _, err := os.Stat(bucketJobConf); err == nil {
		bucketData, err := os.ReadFile(bucketJobConf)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
		}

		err = toml.Unmarshal(bucketData, &config)
		if err != nil {
			return nil, fmt.Errorf("%w: bucket conf %s %w", bucket.ErrInvalidBucketConf, bucketJobConf, err)
		}
	}

	if _, ok := config[job]; !ok {
		config[job] = make(map[string]string)
	}

	return config[job], nil
}

func processJobData(tx *sql.Tx) error {
	jobs, err := data.GetJobs(tx)
	if err != nil {
		return err
	}

	for _, job := range jobs {

		jobKV := make(map[string]string)

		jobKV["job_id"] = workspace.GetHashUUID(job)
		jobKV["name"] = job

		version, err := data.GetJobVersion(tx, job)
		if err != nil {
			return err
		}
		jobKV["version"] = version

		jobSelectors, err := data.GetJobSelectors(tx, job)
		if err != nil {
			return err
		}
		jobKV["selectors"] = strings.Join(jobSelectors, ",")

		minMemory, maxMemory, err := data.GetJobMemoryLimits(tx, job)
		if err != nil {
			return err
		}
		jobKV["min_memory_mb"] = minMemory
		jobKV["max_memory_mb"] = maxMemory

		minCPU, maxCPU, err := data.GetJobCPULimits(tx, job)
		if err != nil {
			return err
		}
		jobKV["min_cpu_mhz"] = minCPU
		jobKV["max_cpu_mhz"] = maxCPU

		jobConfig, err := getJobConf(job)
		if err != nil {
			return err
		}

		jobKV["memory"], err = data.GetJobMemory(tx, job)
		if err != nil {
			return err
		}

		jobKV["cpu"], err = data.GetJobCPU(tx, job)
		if err != nil {
			return err
		}

		namespace := fmt.Sprintf("maand/job/%s", job)
		err = storeKeyValues(tx, namespace, jobKV)
		if err != nil {
			return err
		}

		namespace = fmt.Sprintf("vars/bucket/job/%s", job)
		err = storeKeyValues(tx, namespace, jobConfig)
		if err != nil {
			return err
		}
	}

	for _, job := range removedJobs {
		nss := []string{fmt.Sprintf("maand/job/%s", job), fmt.Sprintf("vars/job/%s", job), fmt.Sprintf("vars/bucket/job/%s", job)}
		jobWorkers, err := data.GetAllocatedWorkers(tx, job)
		if err != nil {
			return err
		}

		for _, workerIP := range jobWorkers {
			nss = append(nss, fmt.Sprintf("maand/job/%s/worker/%s", job, workerIP))
		}

		for _, ns := range nss {
			err := storeKeyValues(tx, ns, make(map[string]string))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func storeKeyValues(tx *sql.Tx, namespace string, keyValues map[string]string) error {
	var availableKeys []string
	for key, value := range keyValues {
		if err := kv.GetKVStore().Put(tx, namespace, key, value, 0); err != nil {
			return err
		}
		availableKeys = append(availableKeys, key)
	}

	allKeys, err := kv.GetKVStore().GetKeys(tx, namespace)
	if err != nil {
		return err
	}

	diffs := utils.Difference(allKeys, availableKeys)
	for _, diff := range diffs {
		err := kv.GetKVStore().Delete(tx, namespace, diff)
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
