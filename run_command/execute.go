package run_command

import (
	"fmt"
	"maand/bucket"
	"maand/data"
	"maand/health_check"
	"maand/utils"
	"maand/worker"
	"os"
	"path"
	"strings"
	"sync"
)

func Execute(workerComma, labelComma string, concurrency int, shCommand string, disableCheck bool, healthcheck bool) error {
	db, err := data.GetDatabase(true)
	utils.Check(err)

	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	utils.Check(err)
	defer func() {
		_ = tx.Rollback()
	}()

	var workers []string

	if len(workerComma) > 0 {
		workersP := strings.Split(workerComma, ",")
		workers = data.GetWorkers(tx, nil)
		diff := utils.Difference(workersP, workers)
		if len(diff) > 0 {
			panic(fmt.Errorf("invalid input, workers not belong to this bucket %v", diff))
		}
		workers = workersP
	}

	if len(workerComma) == 0 && len(labelComma) > 0 {
		labelsP := strings.Split(labelComma, ",")
		workers = data.GetWorkers(tx, labelsP)
	}

	if len(labelComma) == 0 && len(workerComma) == 0 {
		workers = data.GetWorkers(tx, nil)
	}

	if concurrency < 1 {
		utils.Check(fmt.Errorf("concurrency must be at least 1"))
	}

	workers = utils.Unique(workers)

	if !disableCheck {
		data.ValidateBucketUpdateSeq(tx, workers)
	}

	for _, workerIP := range workers {
		worker.KeyScan(workerIP)
	}

	commandFile := path.Join(bucket.WorkspaceLocation, "command.sh")
	if len(shCommand) == 0 {
		if _, err := os.Stat(commandFile); os.IsNotExist(err) {
			utils.Check(fmt.Errorf("run commands required --cmd argument or command file"))
		}
	}

	var wait sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	semaphore := make(chan struct{}, concurrency)

	for _, workerIP := range workers {
		wait.Add(1)
		semaphore <- struct{}{}

		go func(wp string) {
			defer wait.Done()
			defer func() { <-semaphore }()

			var err error
			if len(shCommand) > 0 {
				err = worker.ExecuteCommand(wp, []string{shCommand}, nil)
			} else {
				err = worker.ExecuteFileCommand(wp, commandFile, nil)
			}
			mu.Lock()
			if err != nil {
				errs = append(errs, err)
			}
			mu.Unlock()
		}(workerIP)
	}

	wait.Wait()
	if healthcheck {
		jobs := data.GetJobs(tx)
		for _, job := range jobs {
			err := health_check.Execute(tx, true, job)
			utils.Check(err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
