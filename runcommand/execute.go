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
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

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
			return bucket.UnexpectedError(err)
		}
	} else {
		content = []byte(shCommand)
	}

	err = os.MkdirAll(bucket.TempLocation, os.ModePerm)
	if err != nil {
		return err
	}

	commandFilePath := path.Join(bucket.TempLocation, "command.sh")
	envs := []string{
		fmt.Sprintf("export BUCKET_ID=%s", bucketID),
	}
	fileContent := fmt.Sprintf("%s\n%s", strings.Join(envs, "\n"), content)

	err = os.WriteFile(commandFilePath, []byte(fileContent), 0o644)
	if err != nil {
		return bucket.UnexpectedError(err)
	}
	defer func() {
		_ = os.Remove(commandFilePath)
	}()

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

		err = executeCommand(dockerClient, bucketID, commandFilePath, batch)
		if err != nil {
			return err
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

func executeCommand(dockerClient *bucket.DockerClient, bucketID string, commandFile string, workers []string) error {
	var wait sync.WaitGroup
	errs := make(chan error, len(workers))
	envs := []string{fmt.Sprintf("BUCKET_ID=%s", bucketID)}

	for _, workerIP := range workers {
		wait.Add(1)

		go func(wp string) {
			defer wait.Done()
			err := worker.ExecuteFileCommand(dockerClient, wp, commandFile, envs)
			if err != nil {
				errs <- fmt.Errorf("%w: %s", err, wp)
			}
		}(workerIP)
	}

	wait.Wait()
	close(errs)

	failed := []string{}
	for err := range errs {
		failed = append(failed, err.Error())
	}

	if len(failed) > 0 {
		return bucket.ErrRunCommand
	}

	return nil
}
