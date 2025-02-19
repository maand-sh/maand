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
	if err != nil {
		return data.NewDatabaseError(err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	if err != nil {
		return data.NewDatabaseError(err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	var workers []string

	if len(workerComma) > 0 {
		workersP := strings.Split(workerComma, ",")

		workers, err = data.GetWorkers(tx, nil)
		if err != nil {
			return err
		}

		diff := utils.Difference(workersP, workers)
		if len(diff) > 0 {
			panic(fmt.Errorf("invalid input, workers not belong to this bucket %v", diff))
		}
		workers = workersP
	}

	if len(workerComma) == 0 && len(labelComma) > 0 {
		labelsP := strings.Split(labelComma, ",")
		workers, err = data.GetWorkers(tx, labelsP)
		if err != nil {
			return err
		}
	}

	if len(labelComma) == 0 && len(workerComma) == 0 {
		workers, err = data.GetWorkers(tx, nil)
		if err != nil {
			return err
		}
	}

	if concurrency < 1 {
		utils.Check(fmt.Errorf("concurrency must be at least 1"))
	}

	workers = utils.Unique(workers)

	if !disableCheck {
		err = data.ValidateBucketUpdateSeq(tx, workers)
		if err != nil {
			return err
		}
	}

	for _, workerIP := range workers {
		err = worker.KeyScan(workerIP)
		if err != nil {
			return err
		}
	}

	commandFile := path.Join(bucket.WorkspaceLocation, "command.sh")
	if len(shCommand) == 0 {
		if _, err := os.Stat(commandFile); os.IsNotExist(err) {
			utils.Check(fmt.Errorf("run commands required --cmd argument or command file"))
		}
	}

	var wait sync.WaitGroup
	var mu sync.Mutex
	var errs map[string]error
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
				errs[wp] = err
			}
			mu.Unlock()
		}(workerIP)
	}

	wait.Wait()

	if healthcheck {
		jobs := data.GetJobs(tx)
		for _, job := range jobs {
			err := health_check.HealthCheck(tx, true, job, true)
			if err != nil {
				return err
			}
		}
	}

	if len(errs) > 0 {
		return &RunCommandError{Err: errs}
	}

	return nil
}
