// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"maand/bucket"
	"maand/data"
	"maand/health_check"
	"maand/job_command"
	"maand/kv"
	"maand/utils"
	"maand/worker"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
)

//go:embed runner.py
var runnerPy []byte

//go:embed worker.py
var workerPy []byte

type WorkerJobs struct {
	Job      string `json:"job"`
	Disabled int    `json:"disabled"`
}

type WorkerData struct {
	BucketID  string   `json:"bucket_id"`
	WorkerID  string   `json:"worker_id"`
	WorkerIP  string   `json:"worker_ip"`
	Labels    []string `json:"labels"`
	UpdateSeq int      `json:"update_seq"`
}

type AllocationData struct {
	AllocationID string `json:"allocation_id"`
	Job          string `json:"job"`
	WorkerData
}

func updateCerts(tx *sql.Tx, job, workerIP string) error {
	workerDirPath := bucket.GetTempWorkerPath(workerIP)
	jobDir := path.Join(workerDirPath, "jobs", job)

	rows, err := tx.Query("SELECT name FROM job_certs WHERE job_id = (SELECT job_id FROM job WHERE name = ?)", job)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return data.NewDatabaseError(err)
		}

		certsDir := path.Join(jobDir, "certs")
		err := os.MkdirAll(certsDir, os.ModePerm)
		if err != nil {
			return err
		}

		pubCert, err := kv.GetKVStore().Get(tx, fmt.Sprintf("maand/worker/%s", workerIP), fmt.Sprintf("certs/%s.crt", name))
		if err != nil {
			return err
		}

		err = os.WriteFile(path.Join(certsDir, fmt.Sprintf("%s.crt", name)), []byte(pubCert), os.ModePerm)
		if err != nil {
			return err
		}

		priCert, err := kv.GetKVStore().Get(tx, fmt.Sprintf("maand/worker/%s", workerIP), fmt.Sprintf("certs/%s.key", name))
		if err != nil {
			return err
		}

		err = os.WriteFile(path.Join(certsDir, fmt.Sprintf("%s.key", name)), []byte(priCert), os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

func transpile(tx *sql.Tx, job, workerIP string) error {
	workerDir := bucket.GetTempWorkerPath(workerIP)
	jobDir := path.Join(workerDir, "jobs", job)

	var jobTemplates []string
	err := fs.WalkDir(os.DirFS(jobDir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(d.Name(), ".tpl") {
			jobTemplates = append(jobTemplates, path)
		}

		return nil
	})
	if err != nil {
		return err
	}

	allowedNamespaces := []string{
		"maand",
		"vars/bucket",
		"maand/worker",
		fmt.Sprintf("maand/worker/%s", workerIP),
		fmt.Sprintf("maand/worker/%s/tags", workerIP),
		fmt.Sprintf("maand/job/%s", job),
		fmt.Sprintf("vars/bucket/job/%s", job),
		fmt.Sprintf("vars/job/%s", job),
		fmt.Sprintf("maand/job/%s/worker/%s", job, workerIP),
	}

	funcMap := template.FuncMap{
		"get": func(ns, key string) string {
			if len(utils.Difference([]string{ns}, allowedNamespaces)) > 0 {
				panic(fmt.Sprintf("%s namespace is not available for job %s", ns, job))
			}
			value, err := kv.GetKVStore().Get(tx, ns, key)
			if err != nil {
				panic(fmt.Sprintf("%s, %s is not found", ns, key))
			}
			return value
		},
		"keys": func(ns string) []string {
			if len(utils.Difference([]string{ns}, allowedNamespaces)) > 0 {
				panic(fmt.Sprintf("%s namespace is not available for job %s", ns, job))
			}
			value, err := kv.GetKVStore().GetKeys(tx, ns)
			if err != nil {
				panic(err)
			}
			return value
		},
		"split": strings.Split,
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"join":  strings.Join,
	}

	workerData, err := getWorkerData(tx, workerIP)
	if err != nil {
		return err
	}

	allocID, err := data.GetAllocationID(tx, workerIP, job)
	if err != nil {
		return err
	}

	templateData := AllocationData{
		AllocationID: allocID,
		Job:          job,
		WorkerData:   workerData,
	}

	for _, jobTemplate := range jobTemplates {
		templateAbsPath, err := filepath.Abs(path.Join(jobDir, jobTemplate))
		if err != nil {
			return err
		}

		templateContent, err := os.ReadFile(templateAbsPath)
		if err != nil {
			return err
		}

		tmpl, err := template.New("template").Funcs(funcMap).Parse(string(templateContent))
		if err != nil {
			return err //TODO: handle with template error
		}

		ext := path.Ext(jobTemplate)
		filePath := strings.TrimSuffix(jobTemplate, ext)

		file, err := os.Create(path.Join(jobDir, filePath))
		if err != nil {
			return err
		}

		err = tmpl.Execute(file, templateData)
		if err != nil {
			return err
		}

		err = file.Close()
		if err != nil {
			return err
		}

		err = os.Remove(templateAbsPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func getWorkerData(tx *sql.Tx, workerIP string) (WorkerData, error) {
	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return WorkerData{}, err
	}

	workerID, err := data.GetWorkerID(tx, workerIP)
	if err != nil {
		return WorkerData{}, err
	}

	labels, err := data.GetWorkerLabels(tx, workerID)
	if err != nil {
		return WorkerData{}, err
	}

	updateSeq, err := data.GetUpdateSeq(tx)
	if err != nil {
		return WorkerData{}, err
	}

	deployableWorker := WorkerData{
		BucketID:  bucketID,
		WorkerID:  workerID,
		WorkerIP:  workerIP,
		Labels:    labels,
		UpdateSeq: updateSeq,
	}

	return deployableWorker, nil
}

func prepareWorkersFiles(tx *sql.Tx, workers []string) error {
	for _, workerIP := range workers {

		workerDirPath := bucket.GetTempWorkerPath(workerIP)
		err := os.MkdirAll(workerDirPath, os.ModePerm)
		if err != nil {
			return err
		}

		deployableWorker, err := getWorkerData(tx, workerIP)
		if err != nil {
			return err
		}

		workerData, err := json.MarshalIndent(deployableWorker, "", "   ")
		if err != nil {
			return err
		}

		err = os.WriteFile(path.Join(workerDirPath, "worker.json"), workerData, os.ModePerm)
		if err != nil {
			return err
		}

		allocatedJobs, err := data.GetAllocatedJobs(tx, workerIP)
		if err != nil {
			return err
		}

		var workerJobs = make([]WorkerJobs, 0)
		for _, job := range allocatedJobs {
			disabled, err := data.IsAllocationDisabled(tx, workerIP, job)
			if err != nil {
				return err
			}

			workerJobs = append(workerJobs, WorkerJobs{
				Job:      job,
				Disabled: disabled,
			})
		}

		workerJobsData, err := json.MarshalIndent(workerJobs, "", "   ")
		if err != nil {
			return err
		}

		err = os.WriteFile(path.Join(workerDirPath, "jobs.json"), workerJobsData, os.ModePerm)
		if err != nil {
			return err
		}

		err = os.MkdirAll(path.Join(workerDirPath, "bin"), os.ModePerm)
		if err != nil {
			return err
		}

		err = os.WriteFile(path.Join(workerDirPath, "bin", "runner.py"), runnerPy, os.ModePerm)
		if err != nil {
			return err
		}

		err = os.WriteFile(path.Join(workerDirPath, "bin", "worker.py"), workerPy, os.ModePerm)
		if err != nil {
			return err
		}

		err = os.MkdirAll(path.Join(workerDirPath, "jobs"), os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

func prepareJobsFiles(tx *sql.Tx, jobs []string) error {
	for _, job := range jobs {
		allocatedWorkers, err := data.GetActiveAllocations(tx, job)
		if err != nil {
			return err
		}

		for _, workerIP := range allocatedWorkers {
			workerDirPath := bucket.GetTempWorkerPath(workerIP)

			err = data.CopyJobFiles(tx, job, path.Join(workerDirPath, "jobs"))
			if err != nil {
				return err
			}

			moduleDir := path.Join(workerDirPath, "jobs", job, "_modules")
			if _, err := os.Stat(moduleDir); err == nil {
				err = os.WriteFile(path.Join(moduleDir, "maand.py"), job_command.MaandPy, os.ModePerm)
				if err != nil {
					return err
				}
			}

			err := transpile(tx, job, workerIP)
			if err != nil {
				return err
			}

			err = updateCerts(tx, job, workerIP)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func executePreJobCommands(tx *sql.Tx, jobs []string) error {
	for _, job := range jobs {
		commands, err := data.GetJobCommands(tx, job, "pre_deploy")
		if err != nil {
			return err
		}

		if len(commands) == 0 {
			continue
		}
		for _, command := range commands {
			err := job_command.JobCommand(tx, job, command, "pre_deploy", 1, true)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func executePostJobCommands(tx *sql.Tx, jobs []string) error {
	for _, job := range jobs {
		commands, err := data.GetJobCommands(tx, job, "post_deploy")
		if err != nil {
			return err
		}

		if len(commands) == 0 {
			continue
		}
		for _, command := range commands {
			err := job_command.JobCommand(tx, job, command, "post_deploy", 1, true)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func updateAllocationHash(tx *sql.Tx, jobs []string) error {
	for _, job := range jobs {
		allocatedWorkers, err := data.GetAllocatedWorkers(tx, job)
		if err != nil {
			return err
		}

		for _, workerIP := range allocatedWorkers {
			workerDirPath := bucket.GetTempWorkerPath(workerIP)
			jobDir := path.Join(workerDirPath, "jobs", job)

			allocID, err := data.GetAllocationID(tx, workerIP, job)
			if err != nil {
				return err
			}

			removed, err := data.IsAllocationRemoved(tx, workerIP, job)
			if err != nil {
				return err
			}

			if removed == 1 {
				// remove hash for removed allocations
				err := data.RemoveHash(tx, fmt.Sprintf("%s_allocation", job), allocID)
				if err != nil {
					return err
				}

				_, err = tx.Exec("DELETE FROM allocations WHERE removed = 1 AND job = ? AND worker_ip = ?", job, workerIP)
				if err != nil {
					return data.NewDatabaseError(err)
				}

				continue
			}

			md5, err := utils.CalculateDirMD5(jobDir)
			if err != nil {
				return err
			}

			err = data.UpdateHash(tx, fmt.Sprintf("%s_allocation", job), allocID, md5)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func promoteAllocationHash(tx *sql.Tx, jobs []string) error {
	for _, job := range jobs {
		allocatedWorkers, err := data.GetAllocatedWorkers(tx, job)
		if err != nil {
			return err
		}

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
				_, err := tx.Exec("UPDATE hash SET previous_hash = NULL WHERE namespace = ? AND key = ?", fmt.Sprintf("%s_allocation", job), allocID)
				if err != nil {
					return err
				}
				continue
			}
			err = data.PromoteHash(tx, fmt.Sprintf("%s_allocation", job), allocID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func syncWorkerFiles(bucketID, workerIP string) error {
	err := worker.ExecuteCommand(workerIP, []string{fmt.Sprintf("mkdir -p /opt/worker/%s", bucketID)}, nil)
	if err != nil {
		return err
	}
	return rsync(bucketID, workerIP)
}

func syncWorkers(bucketID string, workers []string, jobs []string, applyRules bool) error {
	var wait sync.WaitGroup
	semaphore := make(chan struct{}, len(workers))

	for _, workerIP := range workers {
		var rsyncMergeLines []string
		if applyRules {
			for _, job := range jobs {
				rsyncMergeLines = append(rsyncMergeLines, fmt.Sprintf("+ jobs/%s\n", job))
			}
			rsyncMergeLines = append(rsyncMergeLines, "- jobs/*\n")
		}

		rsyncMergeFilePath := path.Join(bucket.TempLocation, "workers", fmt.Sprintf("%s.rsync", workerIP))
		err := os.WriteFile(rsyncMergeFilePath, []byte(strings.Join(rsyncMergeLines, "")), os.ModePerm)
		if err != nil {
			return err
		}

		wait.Add(1)
		semaphore <- struct{}{}

		go func(tBucketID, tWorkerIP string) {
			defer wait.Done()
			defer func() { <-semaphore }()
			err = syncWorkerFiles(tBucketID, tWorkerIP)
			if err != nil {
				log.Printf("rsync failed on worker %s", workerIP)
			}
		}(bucketID, workerIP)
	}
	wait.Wait()

	return nil
}

func handleNewAllocations(tx *sql.Tx, bucketID string, jobs []string) error {
	var wait sync.WaitGroup

	for _, job := range jobs {
		wait.Add(1)

		go func(tJob string) {
			defer wait.Done()

			newAllocations, err := data.GetNewAllocations(tx, tJob)
			if err != nil {
				fmt.Print(err.Error())
			}

			if len(newAllocations) > 0 {

				var waitWorker sync.WaitGroup
				var parallelBatchCount = len(newAllocations)

				workerCount := len(newAllocations)
				semaphore := make(chan struct{}, parallelBatchCount) // Limit to 2 workers at a time

				for i := 0; i < workerCount; i += parallelBatchCount {
					batchSize := min(parallelBatchCount, workerCount-i) // Process up to 2 workers at a time
					for j := i; j < batchSize; j++ {
						workerIP := newAllocations[i+j]

						disabled, err := data.IsAllocationDisabled(tx, workerIP, tJob)
						if err != nil {
							fmt.Print(err)
						}

						if disabled == 1 {
							continue
						}

						waitWorker.Add(1)
						semaphore <- struct{}{} // Acquire slot

						go func(tWorkerIP string) {
							defer waitWorker.Done()
							defer func() { <-semaphore }() // Release slot after execution

							err := worker.ExecuteCommand(tWorkerIP, []string{fmt.Sprintf("python3 /opt/worker/%s/bin/runner.py %s start --jobs %s", bucketID, bucketID, tJob)}, nil)
							if err != nil {
								log.Printf("unable to start allocation. job %s, worker %s, reason %v", tJob, tWorkerIP, err)
							}
						}(workerIP)
					}

					waitWorker.Wait()

					err := health_check.HealthCheck(tx, true, tJob, true)
					if err != nil {
						log.Printf("Error in health check: %v", err)
					}
				}
			}
		}(job)
	}

	wait.Wait()

	return nil
}

func handleUpdatedAllocations(tx *sql.Tx, bucketID string, jobs []string) error {
	var wait sync.WaitGroup
	for _, job := range jobs {
		wait.Add(1)

		go func(tJob string) {
			defer wait.Done()

			updatedAllocations, err := data.GetUpdatedAllocations(tx, tJob)
			if err != nil {
				fmt.Print(err.Error())
			}

			if len(updatedAllocations) > 0 {
				parallelBatchCount, err := data.GetUpdateParallelCount(tx, job)
				if err != nil {
					fmt.Print(err.Error())
				}

				var waitWorker sync.WaitGroup
				workerCount := len(updatedAllocations)
				semaphore := make(chan struct{}, parallelBatchCount) // Limit to parallelBatchCount workers at a time

				for i := 0; i < workerCount; i += parallelBatchCount {
					// Ensure you don't exceed the bounds of the updatedAllocations slice
					batchSize := min(parallelBatchCount, workerCount-i)
					for j := 0; j < batchSize; j++ {
						workerIP := updatedAllocations[i+j]
						disabled, err := data.IsAllocationDisabled(tx, workerIP, tJob)
						if err != nil {
							fmt.Print(err)
						}

						if disabled == 1 {
							continue
						}

						waitWorker.Add(1)
						semaphore <- struct{}{} // Acquire slot

						go func(tWorkerIP string) {
							defer waitWorker.Done()
							defer func() { <-semaphore }() // Release slot after execution

							err := worker.ExecuteCommand(tWorkerIP, []string{fmt.Sprintf("python3 /opt/worker/%s/bin/runner.py %s restart --jobs %s", bucketID, bucketID, tJob)}, nil)
							if err != nil {
								log.Printf("unable to restart allocation. job %s, worker %s, reason %v", tJob, tWorkerIP, err)
							}
						}(workerIP)
					}

					waitWorker.Wait()
					err := health_check.HealthCheck(tx, true, tJob, true)
					if err != nil {
						log.Printf("Error during health check for job %s: %v", tJob, err)
					}
				}
			}
		}(job)
	}
	wait.Wait()

	return nil
}

func handleStoppedAllocations(tx *sql.Tx, bucketID string, jobs []string) error {

	stopAllocation := func(tWorkerIP string, tJob string) {
		err := worker.ExecuteCommand(tWorkerIP, []string{fmt.Sprintf("python3 /opt/worker/%s/bin/runner.py %s stop --jobs %s", bucketID, bucketID, tJob)}, nil)
		fmt.Println(err)
		// TODO: error handling
	}

	for _, job := range jobs {
		allocatedWorkers, err := data.GetAllocatedWorkers(tx, job)
		if err != nil {
			return err
		}

		for _, workerIP := range allocatedWorkers {

			removed, err := data.IsAllocationRemoved(tx, workerIP, job)
			if err != nil {
				return err
			}
			disabled, err := data.IsAllocationDisabled(tx, workerIP, job)
			if err != nil {
				return err
			}

			if removed == 1 || disabled == 1 {
				allocID, err := data.GetAllocationID(tx, workerIP, job)
				if err != nil {
					return err
				}

				previousHash, err := data.GetPreviousHash(tx, fmt.Sprintf("%s_allocation", job), allocID)
				if err != nil {
					return err
				}
				if previousHash == "" {
					continue // ignore if already disabled by maand
				}
				go stopAllocation(workerIP, job)
			}
		}

		old, err := data.GetAllocatedWorkers(tx, job)
		if err != nil {
			return err
		}

		if len(allocatedWorkers) != len(old) {
			// run health check only if more than on one active allocation available
			err := health_check.HealthCheck(tx, true, job, true)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func UpdateSeq(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return data.NewDatabaseError(err)
	}

	updateSeq, err := data.GetUpdateSeq(tx)
	if err != nil {
		return err
	}

	updateSeq = updateSeq + 1
	err = data.UpdateSeq(tx, updateSeq)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return data.NewDatabaseError(err)
	}
	return nil
}

func Execute(jobsFilter []string) error {

	db, err := data.GetDatabase(true)
	if err != nil {
		return err
	}

	defer func() {
		_ = db.Close()
		_ = os.RemoveAll(bucket.TempLocation)
	}()

	err = os.RemoveAll(bucket.TempLocation)
	if err != nil {
		return err
	}

	err = UpdateSeq(db)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return data.NewDatabaseError(err)
	}

	newTx := func() error {
		err = tx.Commit()
		if err != nil {
			return data.NewDatabaseError(err)
		}
		tx, err = db.Begin()
		if err != nil {
			return data.NewDatabaseError(err)
		}
		return nil
	}

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}

	maxDeploymentSequence, err := data.GetMaxDeploymentSeq(tx)
	if err != nil {
		return err
	}

	for _, workerIP := range workers {
		err = worker.KeyScan(workerIP)
		if err != nil {
			return err
		}
	}

	// removing all jobs fails on deps seq
	for deploymentSeq := 0; deploymentSeq <= maxDeploymentSequence; deploymentSeq++ {

		var availableJobs = data.GetJobsByDeploymentSeq(tx, deploymentSeq)
		var jobs []string
		for _, job := range availableJobs {
			if len(jobsFilter) > 0 && len(utils.Intersection(jobsFilter, []string{job})) == 0 {
				continue
			}
			jobs = append(jobs, job)
		}

		err = handleStoppedAllocations(tx, bucketID, jobs)
		if err != nil {
			return err
		}

		err = prepareWorkersFiles(tx, workers)
		if err != nil {
			return err
		}

		err = executePreJobCommands(tx, jobs)
		if err != nil {
			return err
		}

		err = prepareJobsFiles(tx, jobs)
		if err != nil {
			return err
		}

		err = syncWorkers(bucketID, workers, jobs, true)
		if err != nil {
			return err
		}

		err = newTx()
		if err != nil {
			return err
		}

		err = updateAllocationHash(tx, jobs)
		if err != nil {
			return err
		}

		err = newTx()
		if err != nil {
			return err
		}

		err = handleNewAllocations(tx, bucketID, jobs)
		if err != nil {
			return err
		}

		err = handleUpdatedAllocations(tx, bucketID, jobs)
		if err != nil {
			return err
		}

		err = executePostJobCommands(tx, jobs)
		if err != nil {
			return err
		}

		err = newTx()
		if err != nil {
			return err
		}

		err = promoteAllocationHash(tx, jobs)
		if err != nil {
			return err
		}

		err = newTx()
		if err != nil {
			return err
		}
	}

	for _, workerIP := range workers {
		jobs, err := data.GetAllocatedJobs(tx, workerIP)
		if err != nil {
			return err
		}

		err = syncWorkers(bucketID, workers, jobs, false)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return data.NewDatabaseError(err)
	}

	if err = data.UpdateJournalModeDefault(db); err != nil {
		return err
	}

	return utils.ExecuteCommand([]string{"sync"})
}
