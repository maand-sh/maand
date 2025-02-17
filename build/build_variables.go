package build

import (
	"database/sql"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"maand/bucket"
	"maand/data"
	"maand/utils"
	"maand/workspace"
	"os"
	"path"
	"strconv"
	"strings"
)

func Variables(tx *sql.Tx) {
	processWorkerData(tx)
	processJobData(tx)
	processLabelData(tx)
	processBucketConf(tx)
}

func processWorkerData(tx *sql.Tx) {
	workers := data.GetWorkers(tx, nil)

	for _, workerIP := range workers {
		workerKV := make(map[string]string)

		workerID := data.GetWorkerID(tx, workerIP)
		workerKV["worker_ip"] = workerIP
		workerKV["worker_id"] = workerID

		workerLabels := data.GetWorkerLabels(tx, workerID)
		workerKV["labels"] = strings.Join(workerLabels, ",")

		for _, label := range workerLabels {
			labelWorkers := data.GetWorkers(tx, []string{label})
			peers := utils.Difference(labelWorkers, []string{workerIP})
			if len(peers) > 0 {
				workerKV[fmt.Sprintf("%s_peers", label)] = strings.Join(peers, ",")
			}
		}

		labels := data.GetLabels(tx)
		for _, label := range labels {
			labelWorkers := data.GetWorkers(tx, []string{label})

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

		availableCPUMhz := data.GetWorkerCPU(tx, workerIP)
		workerKV["worker_cpu_mhz"] = availableCPUMhz
		availableMemoryMb := data.GetWorkerMemory(tx, workerIP)
		workerKV["worker_memory_mb"] = availableMemoryMb
		workerKV["jobs"] = strings.Join(data.GetAllocatedJobs(tx, workerIP), ",")

		workerKVNamespace := fmt.Sprintf("maand/worker/%s", workerIP)
		err := storeKeyValues(tx, workerKVNamespace, workerKV)
		utils.Check(err)

		tags := data.GetWorkerTags(tx, workerID)
		tagsNS := fmt.Sprintf("maand/worker/%s/tags", workerIP)
		err = storeKeyValues(tx, tagsNS, tags)
		utils.Check(err)
	}

	for _, workerIP := range removedWorkers {
		ns := fmt.Sprintf("maand/worker/%s", workerIP)
		keys, _ := utils.GetKVStore().GetKeys(tx, ns)

		for _, key := range keys {
			err := utils.GetKVStore().Delete(tx, ns, key)
			utils.Check(err)
		}
	}
}

func processLabelData(tx *sql.Tx) {
	labels := data.GetLabels(tx)
	workerKV := make(map[string]string)
	for _, label := range labels {
		labelWorkers := data.GetWorkers(tx, []string{label})
		if len(labelWorkers) > 0 {
			workerKV[fmt.Sprintf("%s_label_id", label)] = workspace.GetHashUUID(label)
			workerKV[fmt.Sprintf("%s_nodes", label)] = strings.Join(labelWorkers, ",")
			workerKV[fmt.Sprintf("%s_length", label)] = strconv.Itoa(len(labelWorkers))
		}
		for idx, node := range labelWorkers {
			workerKV[fmt.Sprintf("%s_%d", label, idx)] = node
		}
	}
	err := storeKeyValues(tx, "maand/worker", workerKV)
	utils.Check(err)
}

func processBucketConf(tx *sql.Tx) {
	var bucketConfig = make(map[string]string)

	bucketConfPath := path.Join(bucket.WorkspaceLocation, "bucket.conf")

	if _, err := os.Stat(bucketConfPath); err == nil {
		bucketData, err := os.ReadFile(bucketConfPath)
		utils.Check(err)

		err = toml.Unmarshal(bucketData, &bucketConfig)
		utils.Check(err)
	}

	jobPorts := make(map[string]string)
	availableJobs := data.GetJobs(tx)
	for _, job := range availableJobs {
		rows, err := tx.Query("SELECT name, port FROM job_ports WHERE job_id = (SELECT job_id FROM job WHERE name = ?)", job)
		utils.Check(err)

		for rows.Next() {
			var name, value string
			err := rows.Scan(&name, &value)
			utils.Check(err)

			jobPorts[name] = value
		}
	}

	err := storeKeyValues(tx, "vars/bucket", bucketConfig)
	utils.Check(err)
	err = storeKeyValues(tx, "maand", jobPorts)
	utils.Check(err)
}

func getJobConf(job string) map[string]string {
	var config = make(map[string]map[string]string)

	bucketJobConf := path.Join(bucket.WorkspaceLocation, "bucket.jobs.conf")
	if _, err := os.Stat(bucketJobConf); err == nil {
		bucketData, err := os.ReadFile(bucketJobConf)
		utils.Check(err)

		err = toml.Unmarshal(bucketData, &config)
		utils.Check(err)
	}

	if _, ok := config[job]; !ok {
		config[job] = make(map[string]string)
	}

	return config[job]
}

func processJobData(tx *sql.Tx) {
	for _, job := range data.GetJobs(tx) {

		jobKV := make(map[string]string)

		jobKV["job_id"] = workspace.GetHashUUID(job)
		jobKV["name"] = job
		jobKV["version"] = data.GetJobVersion(tx, job)
		jobKV["selectors"] = strings.Join(data.GetJobSelectors(tx, job), ",")

		minMemory, maxMemory := data.GetJobMemoryLimits(tx, job)
		jobKV["min_memory_mb"] = minMemory
		jobKV["max_memory_mb"] = maxMemory

		minCPU, maxCPU := data.GetJobCPULimits(tx, job)
		jobKV["min_cpu_mhz"] = minCPU
		jobKV["max_cpu_mhz"] = maxCPU

		jobConfig := getJobConf(job)

		if _, ok := jobConfig["memory"]; ok {
			memory, err := utils.ExtractSizeInMB(jobConfig["memory"])
			utils.Check(err)
			jobKV["memory"] = fmt.Sprintf("%v", memory)
		} else {
			_, maxMemoryLimit := data.GetJobMemoryLimits(tx, job)
			jobKV["memory"] = maxMemoryLimit
		}
		if _, ok := jobConfig["cpu"]; ok {
			cpu, err := utils.ExtractCPUFrequencyInMHz(jobConfig["cpu"])
			utils.Check(err)
			jobKV["cpu"] = fmt.Sprintf("%v", cpu)
		} else {
			_, maxCPULimit := data.GetJobCPULimits(tx, job)
			jobKV["cpu"] = maxCPULimit
		}

		namespace := fmt.Sprintf("maand/job/%s", job)
		err := storeKeyValues(tx, namespace, jobKV)
		utils.Check(err)

		namespace = fmt.Sprintf("vars/bucket/job/%s", job)
		err = storeKeyValues(tx, namespace, jobConfig)
		utils.Check(err)
	}

	for _, job := range removedJobs {
		nss := []string{fmt.Sprintf("maand/job/%s", job), fmt.Sprintf("vars/job/%s", job)}
		for _, ns := range nss {
			err := storeKeyValues(tx, ns, make(map[string]string))
			utils.Check(err)
		}
	}
}

func storeKeyValues(tx *sql.Tx, namespace string, kv map[string]string) error {
	var availableKeys []string
	for key, value := range kv {
		if err := utils.GetKVStore().Put(tx, namespace, key, value, 0); err != nil {
			return fmt.Errorf("failed to store key-value pair (%s: %s): %w", key, value, err)
		}
		availableKeys = append(availableKeys, key)
	}

	allKeys, err := utils.GetKVStore().GetKeys(tx, namespace)
	utils.Check(err)

	diffs := utils.Difference(allKeys, availableKeys)
	for _, diff := range diffs {
		err := utils.GetKVStore().Delete(tx, namespace, diff)
		utils.Check(err)
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
