// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package jobcontrol provides interfaces to work with jobcontrol
package jobcontrol

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"maand/bucket"
	"maand/data"
	"maand/healthcheck"
	"maand/jobcommand"
	"maand/utils"
	"maand/worker"
)

func Execute(jobsCSV, workersCSV, target string, healthCheck bool) error {
	db, err := data.GetDatabase(true)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	tx, err := db.Begin()
	if err != nil {
		return data.NewDatabaseError(err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

	dockerClient, err := bucket.SetupBucketContainer(bucketID)
	if err != nil {
		return err
	}
	defer func() {
		_ = dockerClient.Stop()
	}()

	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}

	err = data.ValidateBucketUpdateSeq(tx, dockerClient, workers)
	if err != nil {
		return err
	}

	var workersFilter []string
	if len(workersCSV) > 0 {
		workersFilter = strings.Split(workersCSV, ",")
	}

	var jobsFilter []string
	if len(jobsCSV) > 0 {
		jobsFilter = strings.Split(jobsCSV, ",")
	}

	jobsFilter = utils.Unique(jobsFilter)
	workersFilter = utils.Unique(workersFilter)

	allJobs, err := data.GetAllAllocatedJobs(tx)
	if err != nil {
		return err
	}

	if len(jobsFilter) > 0 && len(utils.Intersection(allJobs, jobsFilter)) == 0 {
		return fmt.Errorf("invalid input, jobs %v", jobsFilter)
	}

	allWorkers, err := data.GetAllWorkers(tx)
	if err != nil {
		return err
	}

	if len(workersFilter) > 0 && len(utils.Intersection(allWorkers, workersFilter)) == 0 {
		return fmt.Errorf("invalid input, workers %v", workersFilter)
	}

	maxDeploymentSequence, err := data.GetMaxDeploymentSeq(tx)
	if err != nil {
		return err
	}

	// removing all jobs fails on deps seq
	for deploymentSeq := 0; deploymentSeq <= maxDeploymentSequence; deploymentSeq++ {

		var selectedJobs []string
		jobs, err := data.GetJobsByDeploymentSeq(tx, deploymentSeq)
		if err != nil {
			return err
		}

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

				allocatedWorkers, err := data.GetActiveAllocations(tx, tJob)
				if err != nil {
					fmt.Println(err)
				}

				commands, err := data.GetJobCommands(tx, tJob, "job_control")
				if err != nil {
					fmt.Println(err)
				}

				if len(commands) > 0 {
					for _, command := range commands {
						err = jobcommand.JobCommand(tx, dockerClient, tJob, command, "job_control", len(allocatedWorkers), true, []string{})
						if err != nil {
							fmt.Printf("job %s, job_control command is failed, %s, %v\n", tJob, command, err)
						}
					}

					if healthCheck {
						err = healthcheck.HealthCheck(tx, dockerClient, true, tJob, true)
						if err != nil {
							fmt.Printf("job %s, health_check failded, %v", tJob, err)
						}
					}
					return
				}

				parallelBatchCount, err := data.GetUpdateParallelCount(tx, tJob)
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
							err = worker.ExecuteCommand(dockerClient, tWorkerIP, []string{fmt.Sprintf("python3 /opt/worker/%s/bin/runner.py %s %s --jobs %s", bucketID, bucketID, target, tJob)}, nil)
							if err != nil {
								fmt.Printf("%s worker is failed\n", tWorkerIP)
							}
						}(workerIP)
					}

					waitWorker.Wait()
					if healthCheck {
						err = healthcheck.HealthCheck(tx, dockerClient, true, tJob, true)
						if err != nil {
							fmt.Println(err)
						}
					}
				}
			}(job)
		}

		wait.Wait()
	}

	if err := tx.Commit(); err != nil {
		return data.NewDatabaseError(err)
	}

	return nil
}
