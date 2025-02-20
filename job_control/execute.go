// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package job_control

import (
	"fmt"
	"log"
	"maand/data"
	"maand/health_check"
	"maand/utils"
	"maand/worker"
	"strings"
	"sync"
)

func Execute(jobsComma, workersComma, target string, healthCheck bool) error {
	db, err := data.GetDatabase(true)
	utils.Check(err)

	tx, err := db.Begin()
	utils.Check(err)
	defer func() {
		_ = tx.Rollback()
	}()

	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}

	err = data.ValidateBucketUpdateSeq(tx, workers)
	if err != nil {
		return err
	}

	var workersFilter []string
	if len(workersComma) > 0 {
		workersFilter = strings.Split(workersComma, ",")
	}

	var jobsFilter []string
	if len(jobsComma) > 0 {
		jobsFilter = strings.Split(jobsComma, ",")
	}

	jobsFilter = utils.Unique(jobsFilter)
	workersFilter = utils.Unique(workersFilter)

	maxDeploymentSequence, err := data.GetMaxDeploymentSeq(tx)
	if err != nil {
		return err
	}

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

	// removing all jobs fails on deps seq
	for deploymentSeq := 0; deploymentSeq <= maxDeploymentSequence; deploymentSeq++ {

		var selectedJobs []string
		var jobs = data.GetJobsByDeploymentSeq(tx, deploymentSeq)

		if len(jobsFilter) > 0 {
			for _, job := range jobs {
				if len(utils.Intersection(jobsFilter, []string{job})) == 1 {
					selectedJobs = append(selectedJobs, job)
				}
			}
		} else {
			selectedJobs = append(selectedJobs, jobs...)
		}

		var wait sync.WaitGroup

		for _, job := range selectedJobs {
			wait.Add(1)

			go func(tJob string) {
				defer wait.Done()

				allocatedWorkers, err := data.GetActiveAllocations(tx, job)
				if err != nil {
					fmt.Println(err)
				}

				for _, workerIP := range allocatedWorkers {
					err = worker.KeyScan(workerIP)
					if err != nil {
						log.Println(err)
					}
				}

				parallelBatchCount, err := data.GetUpdateParallelCount(tx, job)
				if err != nil {
					log.Println(err)
				}

				workerCount := len(allocatedWorkers)
				semaphore := make(chan struct{}, parallelBatchCount) // Limit to 2 workers at a time

				var waitWorker sync.WaitGroup

				for i := 0; i < workerCount; i += parallelBatchCount {

					batchSize := min(parallelBatchCount, workerCount-i) // Process up to 2 workers at a time
					for j := 0; j < batchSize; j++ {
						workerIP := allocatedWorkers[i+j]
						if len(workersFilter) > 0 && len(utils.Intersection(workersFilter, []string{workerIP})) == 0 {
							continue
						}

						waitWorker.Add(1)
						semaphore <- struct{}{} // Acquire slot

						go func(tWorkerIP string) {
							defer waitWorker.Done()
							defer func() { <-semaphore }() // Release slot after execution
							err = worker.ExecuteCommand(workerIP, []string{fmt.Sprintf("python3 /opt/worker/%s/bin/runner.py %s %s --jobs %s", bucketID, bucketID, target, job)}, nil)
							if err != nil {
								fmt.Println(err)
							}
						}(workerIP)
					}

					waitWorker.Wait()
					if healthCheck {
						err = health_check.HealthCheck(tx, true, job, true)
						utils.Check(err)
					}
				}

			}(job)
		}

		wait.Wait()
	}

	if err := tx.Commit(); err != nil {
		return data.NewDatabaseError(err)
	}

	if err = data.UpdateJournalModeDefault(db); err != nil {
		return err
	}

	return nil
}
