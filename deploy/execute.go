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

func updateCerts(tx *sql.Tx, job, workerIP string) {
	workerDirPath := bucket.GetTempWorkerPath(workerIP)
	jobDir := path.Join(workerDirPath, "jobs", job)

	rows, err := tx.Query("SELECT name FROM job_certs WHERE job_id = (SELECT job_id FROM job WHERE name = ?)", job)
	utils.Check(err)

	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		utils.Check(err)

		err := os.MkdirAll(path.Join(jobDir, "certs"), os.ModePerm)
		utils.Check(err)

		pubCert, err := utils.GetKVStore().Get(tx, fmt.Sprintf("maand/worker/%s", workerIP), fmt.Sprintf("certs/%s.crt", name))
		utils.Check(err)

		err = os.WriteFile(path.Join(jobDir, "certs", fmt.Sprintf("%s.crt", name)), []byte(pubCert), os.ModePerm)
		utils.Check(err)

		priCert, err := utils.GetKVStore().Get(tx, fmt.Sprintf("maand/worker/%s", workerIP), fmt.Sprintf("certs/%s.key", name))
		utils.Check(err)

		err = os.WriteFile(path.Join(jobDir, "certs", fmt.Sprintf("%s.key", name)), []byte(priCert), os.ModePerm)
		utils.Check(err)
	}
}

func transpile(tx *sql.Tx, job, workerIP string) {
	workerDir := bucket.GetTempWorkerPath(workerIP)
	jobDir := path.Join(workerDir, "jobs", job)

	var jobTemplates []string
	err := fs.WalkDir(os.DirFS(jobDir), ".", func(path string, d fs.DirEntry, err error) error {
		utils.Check(err)
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".tpl") {
			jobTemplates = append(jobTemplates, path)
		}
		return nil
	})
	utils.Check(err)

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
				utils.Check(fmt.Errorf("%s namespace is not available for this job", ns))
			}
			value, err := utils.GetKVStore().Get(tx, ns, key)
			if err != nil {
				utils.Check(fmt.Errorf("%s, %s is not found", ns, key))
			}
			return value
		},
		"keys": func(ns string) []string {
			if len(utils.Difference([]string{ns}, allowedNamespaces)) > 0 {
				utils.Check(fmt.Errorf("%s namespace is not available for this job", ns))
			}
			value, _ := utils.GetKVStore().GetKeys(tx, ns)
			return value
		},
		"split": strings.Split,
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"join":  strings.Join,
	}

	workerData := getWorkerData(tx, workerIP)
	templateData := AllocationData{
		AllocationID: data.GetAllocationID(tx, workerIP, job),
		Job:          job,
		WorkerData:   workerData,
	}

	for _, jobTemplate := range jobTemplates {
		templateAbsPath, _ := filepath.Abs(path.Join(jobDir, jobTemplate))
		templateContent, _ := os.ReadFile(templateAbsPath)

		tmpl, err := template.New("template").Funcs(funcMap).Parse(string(templateContent))
		utils.Check(err)

		ext := path.Ext(jobTemplate)
		filePath := strings.TrimSuffix(jobTemplate, ext)

		file, err := os.Create(path.Join(jobDir, filePath))
		utils.Check(err)

		err = tmpl.Execute(file, templateData)
		utils.Check(err)

		err = file.Close()
		utils.Check(err)

		err = os.Remove(templateAbsPath)
		utils.Check(err)
	}
}

func getWorkerData(tx *sql.Tx, workerIP string) WorkerData {
	bucketID := data.GetBucketID(tx)
	workerID := data.GetWorkerID(tx, workerIP)
	labels := data.GetWorkerLabels(tx, workerID)
	updateSeq := data.GetUpdateSeq(tx)

	deployableWorker := WorkerData{
		BucketID:  bucketID,
		WorkerID:  workerID,
		WorkerIP:  workerIP,
		Labels:    labels,
		UpdateSeq: updateSeq,
	}

	return deployableWorker
}

func prepareWorkersFiles(tx *sql.Tx, workers []string) {
	for _, workerIP := range workers {
		workerDirPath := bucket.GetTempWorkerPath(workerIP)
		err := os.MkdirAll(workerDirPath, os.ModePerm)
		utils.Check(err)

		deployableWorker := getWorkerData(tx, workerIP)

		workerData, err := json.MarshalIndent(deployableWorker, "", "   ")
		utils.Check(err)

		err = os.WriteFile(path.Join(workerDirPath, "worker.json"), workerData, os.ModePerm)
		utils.Check(err)

		var workerJobs = make([]WorkerJobs, 0)
		allocatedJobs := data.GetAllocatedJobs(tx, workerIP)
		for _, job := range allocatedJobs {
			workerJobs = append(workerJobs, WorkerJobs{
				Job:      job,
				Disabled: data.IsAllocationDisabled(tx, workerIP, job),
			})
		}
		workerJobsData, err := json.MarshalIndent(workerJobs, "", "   ")
		utils.Check(err)

		err = os.WriteFile(path.Join(workerDirPath, "jobs.json"), workerJobsData, os.ModePerm)
		utils.Check(err)

		err = os.MkdirAll(path.Join(workerDirPath, "bin"), os.ModePerm)
		utils.Check(err)

		err = os.WriteFile(path.Join(workerDirPath, "bin", "runner.py"), runnerPy, os.ModePerm)
		utils.Check(err)

		err = os.WriteFile(path.Join(workerDirPath, "bin", "worker.py"), workerPy, os.ModePerm)
		utils.Check(err)

		err = os.MkdirAll(path.Join(workerDirPath, "jobs"), os.ModePerm)
		utils.Check(err)
	}
}

func prepareJobsFiles(tx *sql.Tx, jobs []string) {
	for _, job := range jobs {
		allocatedWorkers := data.GetActiveAllocations(tx, job)
		for _, workerIP := range allocatedWorkers {
			workerDirPath := bucket.GetTempWorkerPath(workerIP)
			data.CopyJobFiles(tx, job, path.Join(workerDirPath, "jobs"))
			moduleDir := path.Join(workerDirPath, "jobs", job, "_modules")
			if _, err := os.Stat(moduleDir); err == nil {
				err = os.WriteFile(path.Join(moduleDir, "maand.py"), job_command.MaandPy, os.ModePerm)
				utils.Check(err)
			}
			transpile(tx, job, workerIP)
			updateCerts(tx, job, workerIP)
		}
	}
}

func executePreJobCommands(tx *sql.Tx, jobs []string) {
	for _, job := range jobs {
		commands := data.GetJobCommands(tx, job, "pre_deploy")
		if len(commands) == 0 {
			continue
		}
		for _, command := range commands {
			err := job_command.Execute(tx, job, command, "pre_deploy", 1)
			utils.Check(err)
		}
	}
}

func executePostJobCommands(tx *sql.Tx, jobs []string) {
	for _, job := range jobs {
		commands := data.GetJobCommands(tx, job, "post_deploy")
		if len(commands) == 0 {
			continue
		}
		for _, command := range commands {
			err := job_command.Execute(tx, job, command, "post_deploy", 1)
			utils.Check(err)
		}
	}
}

func updateAllocationHash(tx *sql.Tx, jobs []string) {
	for _, job := range jobs {
		allocatedWorkers := data.GetAllocatedWorkers(tx, job)
		for _, workerIP := range allocatedWorkers {
			workerDirPath := bucket.GetTempWorkerPath(workerIP)
			jobDir := path.Join(workerDirPath, "jobs", job)
			allocID := data.GetAllocationID(tx, workerIP, job)
			if data.IsAllocationRemoved(tx, workerIP, job) == 1 {
				// remove hash for removed allocations
				utils.RemoveHash(tx, fmt.Sprintf("%s_allocation", job), allocID)
				_, err := tx.Exec("DELETE FROM allocations WHERE removed = 1 AND job = ? AND worker_ip = ?", job, workerIP)
				utils.Check(err)
				continue
			}
			md5, err := utils.CalculateDirMD5(jobDir)
			utils.Check(err)

			utils.UpdateHash(tx, fmt.Sprintf("%s_allocation", job), allocID, md5)
		}
	}
}

func promoteAllocationHash(tx *sql.Tx, jobs []string) {
	for _, job := range jobs {
		allocatedWorkers := data.GetAllocatedWorkers(tx, job)
		for _, workerIP := range allocatedWorkers {
			allocID := data.GetAllocationID(tx, workerIP, job)
			if data.IsAllocationDisabled(tx, workerIP, job) == 1 {
				_, err := tx.Exec("UPDATE hash SET previous_hash = NULL WHERE namespace = ? AND key = ?", fmt.Sprintf("%s_allocation", job), allocID)
				utils.Check(err)
				continue
			}
			utils.PromoteHash(tx, fmt.Sprintf("%s_allocation", job), allocID)
		}
	}
}

func syncWorkerFiles(bucketID, workerIP string) {
	err := worker.ExecuteCommand(workerIP, []string{fmt.Sprintf("mkdir -p /opt/worker/%s", bucketID)}, nil)
	utils.Check(err)
	rsync(bucketID, workerIP)
}

func syncWorkers(bucketID string, workers []string, jobs []string, applyRules bool) {
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
		utils.Check(err)

		wait.Add(1)
		semaphore <- struct{}{}

		go func(tBucketID, tWorkerIP string) {
			defer wait.Done()
			defer func() { <-semaphore }()
			syncWorkerFiles(tBucketID, tWorkerIP)
		}(bucketID, workerIP)
	}
	wait.Wait()
}

func handleNewAllocations(tx *sql.Tx, bucketID string, jobs []string) {
	var wait sync.WaitGroup

	for _, job := range jobs {
		wait.Add(1)

		go func(tJob string) {
			defer wait.Done()

			newAllocations := data.GetNewAllocations(tx, tJob)
			if len(newAllocations) > 0 {

				var waitWorker sync.WaitGroup
				var parallelBatchCount = len(newAllocations)

				workerCount := len(newAllocations)
				semaphore := make(chan struct{}, parallelBatchCount) // Limit to 2 workers at a time

				for i := 0; i < workerCount; i += parallelBatchCount {
					batchSize := min(parallelBatchCount, workerCount-i) // Process up to 2 workers at a time
					for j := i; j < batchSize; j++ {
						workerIP := newAllocations[i+j]

						if data.IsAllocationDisabled(tx, workerIP, tJob) == 1 {
							continue
						}

						waitWorker.Add(1)
						semaphore <- struct{}{} // Acquire slot

						go func(tWorkerIP string) {
							defer waitWorker.Done()
							defer func() { <-semaphore }() // Release slot after execution

							err := worker.ExecuteCommand(tWorkerIP, []string{fmt.Sprintf("python3 /opt/worker/%s/bin/runner.py %s start --jobs %s", bucketID, bucketID, tJob)}, nil)
							if err != nil {
								log.Printf("Error executing command on worker %s: %v", tWorkerIP, err)
							}
						}(workerIP)
					}

					waitWorker.Wait()
					err := health_check.Execute(tx, true, tJob)
					if err != nil {
						log.Printf("Error in health check: %v", err)
					}
				}
			}
		}(job)
	}
	wait.Wait()
}

func handleUpdatedAllocations(tx *sql.Tx, bucketID string, jobs []string) {
	var wait sync.WaitGroup
	for _, job := range jobs {
		wait.Add(1)

		go func(tJob string) {
			defer wait.Done()

			updatedAllocations := data.GetUpdatedAllocations(tx, tJob)
			if len(updatedAllocations) > 0 {
				parallelBatchCount := data.GetUpdateParallelCount(tx, job)

				var waitWorker sync.WaitGroup
				workerCount := len(updatedAllocations)
				semaphore := make(chan struct{}, parallelBatchCount) // Limit to parallelBatchCount workers at a time

				for i := 0; i < workerCount; i += parallelBatchCount {
					// Ensure you don't exceed the bounds of the updatedAllocations slice
					batchSize := min(parallelBatchCount, workerCount-i)
					for j := 0; j < batchSize; j++ {
						workerIP := updatedAllocations[i+j]
						if data.IsAllocationDisabled(tx, workerIP, tJob) == 1 {
							continue
						}

						waitWorker.Add(1)
						semaphore <- struct{}{} // Acquire slot

						go func(tWorkerIP string) {
							defer waitWorker.Done()
							defer func() { <-semaphore }() // Release slot after execution

							err := worker.ExecuteCommand(tWorkerIP, []string{fmt.Sprintf("python3 /opt/worker/%s/bin/runner.py %s restart --jobs %s", bucketID, bucketID, tJob)}, nil)
							if err != nil {
								log.Printf("Error executing command on worker %s: %v", tWorkerIP, err)
							}
						}(workerIP)
					}

					waitWorker.Wait()
					err := health_check.Execute(tx, true, tJob)
					if err != nil {
						log.Printf("Error during health check for job %s: %v", tJob, err)
					}
				}
			}
		}(job)
	}
	wait.Wait()
}

func handleStoppedAllocations(tx *sql.Tx, bucketID string, jobs []string) {
	for _, job := range jobs {
		allocatedWorkers := data.GetAllocatedWorkers(tx, job)
		for _, workerIP := range allocatedWorkers {
			if data.IsAllocationRemoved(tx, workerIP, job) == 1 || data.IsAllocationDisabled(tx, workerIP, job) == 1 {
				allocID := data.GetAllocationID(tx, workerIP, job)
				if utils.GetPreviousHash(tx, fmt.Sprintf("%s_allocation", job), allocID) == "" {
					continue // ignore if already disabled by maand
				}
				go func(tWorkerIP string, tJob string) {
					err := worker.ExecuteCommand(tWorkerIP, []string{fmt.Sprintf("python3 /opt/worker/%s/bin/runner.py %s stop --jobs %s", bucketID, bucketID, tJob)}, nil)
					utils.Check(err)
				}(workerIP, job)
			}
		}
		if len(allocatedWorkers) != len(data.GetAllocatedWorkers(tx, job)) {
			// run health check only if more than on one active allocation available
			err := health_check.Execute(tx, true, job)
			utils.Check(err)
		}
	}
}

func UpdateSeq(db *sql.DB) {
	tx, err := db.Begin()
	utils.Check(err)

	updateSeq := data.GetUpdateSeq(tx)
	updateSeq = updateSeq + 1
	data.UpdateSeq(tx, updateSeq)

	err = tx.Commit()
	utils.Check(err)
}

func Execute(jobsFilter []string) {

	db, err := data.GetDatabase(true)
	utils.Check(err)
	defer func() {
		_ = db.Close()
		_ = os.RemoveAll(bucket.TempLocation)
	}()

	err = os.RemoveAll(bucket.TempLocation)
	utils.Check(err)

	UpdateSeq(db)

	tx, err := db.Begin()
	utils.Check(err)

	newTx := func() {
		err = tx.Commit()
		utils.Check(err)
		tx, err = db.Begin()
		utils.Check(err)
	}

	bucketID := data.GetBucketID(tx)
	workers := data.GetWorkers(tx, nil)
	maxDeploymentSequence := data.GetMaxDeploymentSeq(tx)

	for _, workerIP := range workers {
		worker.KeyScan(workerIP)
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

		handleStoppedAllocations(tx, bucketID, jobs)

		prepareWorkersFiles(tx, workers)
		executePreJobCommands(tx, jobs)
		prepareJobsFiles(tx, jobs)
		syncWorkers(bucketID, workers, jobs, true)

		newTx()
		updateAllocationHash(tx, jobs)
		newTx()

		handleNewAllocations(tx, bucketID, jobs)
		handleUpdatedAllocations(tx, bucketID, jobs)
		executePostJobCommands(tx, jobs)

		newTx()
		promoteAllocationHash(tx, jobs)
		newTx()
	}

	for _, workerIP := range workers {
		jobs := data.GetAllocatedJobs(tx, workerIP)
		syncWorkers(bucketID, workers, jobs, false)
	}

	err = tx.Commit()
	utils.Check(err)

	_ = utils.ExecuteCommand([]string{"sync"})
}
