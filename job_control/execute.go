package job_control

import (
	"fmt"
	"maand/data"
	"maand/health_check"
	"maand/utils"
	"maand/worker"
	"strings"
	"sync"
)

func Execute(jobsComma, workersComma, target string, healthCheck bool) {
	db, err := data.GetDatabase(true)
	utils.Check(err)

	tx, err := db.Begin()
	utils.Check(err)
	defer func() {
		_ = tx.Rollback()
	}()

	workers := data.GetWorkers(tx, nil)
	data.ValidateBucketUpdateSeq(tx, workers)

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

	maxDeploymentSequence := data.GetMaxDeploymentSeq(tx)
	bucketID := data.GetBucketID(tx)

	// removing all jobs fails on deps seq
	for deploymentSeq := 0; deploymentSeq <= maxDeploymentSequence; deploymentSeq++ {

		var jobs = data.GetJobsByDeploymentSeq(tx, deploymentSeq)
		for _, job := range jobs {
			if len(jobsFilter) > 0 && len(utils.Intersection(jobsFilter, []string{job})) == 0 {
				continue
			}
			jobs = append(jobs, job)
		}

		var wait sync.WaitGroup

		for _, job := range jobs {
			wait.Add(1)

			go func(tJob string) {
				defer wait.Done()

				allocatedWorkers := data.GetActiveAllocations(tx, job)
				for _, workerIP := range allocatedWorkers {
					worker.KeyScan(workerIP)
				}

				var waitWorker sync.WaitGroup
				var parallelBatchCount = data.GetUpdateParallelCount(tx, job)
				workerCount := len(allocatedWorkers)
				semaphore := make(chan struct{}, parallelBatchCount) // Limit to 2 workers at a time

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
							utils.Check(err)
						}(workerIP)
					}

					waitWorker.Wait()
					if healthCheck {
						err = health_check.Execute(tx, true, job)
						utils.Check(err)
					}
				}

			}(job)
		}

		wait.Wait()
	}
}
