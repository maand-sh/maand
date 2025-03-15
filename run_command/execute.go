// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

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

func Execute(workerCSV, labelCSV string, concurrency int, shCommand string, healthcheck bool) error {
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

	if len(workerCSV) > 0 {
		workersArgs := strings.Split(workerCSV, ",")

		workers, err = data.GetWorkers(tx, nil)
		if err != nil {
			return err
		}

		diff := utils.Difference(workersArgs, workers)
		if len(diff) > 0 {
			panic(fmt.Errorf("invalid input, workers not belong to this bucket %v", diff))
		}
		workers = workersArgs
	}

	if len(workerCSV) == 0 && len(labelCSV) > 0 {
		labelsP := strings.Split(labelCSV, ",")
		workers, err = data.GetWorkers(tx, labelsP)
		if err != nil {
			return err
		}
	}

	if len(labelCSV) == 0 && len(workerCSV) == 0 {
		workers, err = data.GetWorkers(tx, nil)
		if err != nil {
			return err
		}
	}

	if concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1")
	}

	workers = utils.Unique(workers)

	var content []byte
	commandFile := path.Join(bucket.WorkspaceLocation, "command.sh")
	if len(shCommand) == 0 {
		if _, err := os.Stat(commandFile); os.IsNotExist(err) {
			return fmt.Errorf("run commands required argument input or command file")
		}
		content, err = os.ReadFile(commandFile)
		if err != nil {
			return fmt.Errorf("unable to read command file, %v", err)
		}
	} else {
		content = []byte(shCommand)
	}

	err = os.MkdirAll(bucket.TempLocation, os.ModePerm)
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(bucket.TempLocation, "command.sh"), content, 0644)
	if err != nil {
		return fmt.Errorf("unable to copy command file, %v", err)
	}
	commandFile = "command.sh"

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

	var wait sync.WaitGroup
	var mu sync.Mutex
	var errs = make(map[string]error)
	semaphore := make(chan struct{}, concurrency)

	for _, workerIP := range workers {
		wait.Add(1)
		semaphore <- struct{}{}

		go func(wp string) {
			defer wait.Done()
			defer func() { <-semaphore }()

			err := worker.ExecuteFileCommand(dockerClient, wp, commandFile, nil)
			mu.Lock()
			if err != nil {
				errs[wp] = err
			}
			mu.Unlock()
		}(workerIP)
	}

	wait.Wait()

	if healthcheck {
		jobs, err := data.GetJobs(tx)
		if err != nil {
			return err
		}

		for _, job := range jobs {
			err := health_check.HealthCheck(tx, dockerClient, true, job, true)
			if err != nil {
				return err
			}
		}
	}

	if len(errs) > 0 {
		return &RunCommandError{Errs: errs}
	}

	return nil
}
