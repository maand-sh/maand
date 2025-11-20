// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package runcommand provides interfaces to work with run command
package runcommand

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"maand/bucket"
	"maand/data"
	"maand/healthcheck"
	"maand/utils"
	"maand/worker"
)

func Execute(workerCSV, labelCSV string, concurrency int, shCommand string, runHealthcheck bool) error {
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
		return errors.New("concurrency must be at least 1")
	}

	workers = utils.Unique(workers)

	var content []byte
	commandFile := path.Join(bucket.WorkspaceLocation, "command.sh")
	if len(shCommand) == 0 {
		_, err := os.Stat(commandFile)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New("command/command file is required")
			}
			return err
		}
		content, err = os.ReadFile(commandFile)
		if err != nil {
			return bucket.NewUnexpectedError(err)
		}
	} else {
		content = []byte(shCommand)
	}

	err = os.MkdirAll(bucket.TempLocation, os.ModePerm)
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(bucket.TempLocation, "command.sh"), content, 0o644)
	if err != nil {
		return bucket.NewUnexpectedError(err)
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

	iterator := utils.NewStringIterator(workers, concurrency)
	for {
		batch, hasMore := iterator()
		if !hasMore {
			break
		}

		var wait sync.WaitGroup
		var mu sync.Mutex
		errs := make(map[string]error)

		for _, workerIP := range batch {
			wait.Add(1)

			go func(wp string) {
				defer wait.Done()
				err := worker.ExecuteFileCommand(dockerClient, wp, commandFile, nil)
				mu.Lock()
				if err != nil {
					errs[wp] = err
				}
				mu.Unlock()
			}(workerIP)
		}

		wait.Wait()

		if len(errs) > 0 {
			failedWorkers := []string{}
			for wp := range errs {
				failedWorkers = append(failedWorkers, wp)
			}
			return fmt.Errorf("%s worker(s) failed", strings.Join(failedWorkers, ","))
		}

		if runHealthcheck {
			time.Sleep(10 * time.Second)
			jobs, err := data.GetJobs(tx)
			if err != nil {
				return err
			}

			for _, job := range jobs {
				err := healthcheck.HealthCheck(tx, dockerClient, true, job, true)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
