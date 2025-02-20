// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package job_command

import (
	"database/sql"
	_ "embed"
	"fmt"
	"maand/bucket"
	"maand/data"
	"maand/kv"
	"maand/utils"
	"maand/worker"
	"os"
	"os/exec"
	"path"
	"strconv"
	"sync"
)

//go:embed maand.py
var MaandPy []byte

func JobCommand(tx *sql.Tx, job, command, event string, concurrency int, verbose bool) error {
	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}

	for _, workerIP := range workers {
		err := worker.KeyScan(workerIP)
		if err != nil {
			return err
		}
	}

	// TODO: bucket validation

	allowedCommands, err := data.GetJobCommands(tx, job, event)
	if err != nil {
		return err
	}

	if len(utils.Intersection([]string{command}, allowedCommands)) == 0 {
		return &JobCommandNotFoundError{Command: command, Job: job, Event: event}
	}

	workers, err = data.GetActiveAllocations(tx, job)
	if err != nil {
		return err
	}

	for _, workerIP := range workers {

		workerDir := bucket.GetTempWorkerPath(workerIP)
		err := os.MkdirAll(workerDir, os.ModePerm)
		if err != nil {
			return err
		}

		err = data.CopyJobFiles(tx, job, path.Join(workerDir, "jobs"))
		if err != nil {
			return err
		}

		moduleDir := path.Join(workerDir, "jobs", job, "_modules")
		if _, err = os.Stat(moduleDir); err == nil {
			err = os.WriteFile(path.Join(moduleDir, "maand.py"), MaandPy, os.ModePerm)
			if err != nil {
				return err
			}
		}
	}

	allocErrors := map[string]error{}
	var semaphore = make(chan struct{}, concurrency)
	var wait sync.WaitGroup
	var mu sync.Mutex

	for workerIndex, workerIP := range workers {
		wait.Add(1)
		semaphore <- struct{}{}

		disabled, err := data.IsAllocationDisabled(tx, workerIP, job)
		if err != nil {
			return err
		}

		go func(tWorkerIndex int, tWorkerIP string, tDisabled int) {
			defer wait.Done()
			defer func() { <-semaphore }()

			err := runAllocationCommand(job, tWorkerIndex, tWorkerIP, tDisabled, command, event, verbose)
			if err != nil {
				mu.Lock()
				allocErrors[tWorkerIP] = err
				mu.Unlock()
			}
		}(workerIndex, workerIP, disabled)
	}

	_ = utils.ExecuteCommand([]string{"sync"})

	wait.Wait()
	close(semaphore)

	if len(allocErrors) > 0 {
		return &JobCommandError{Job: job, Command: command, Err: allocErrors}
	}

	return nil
}

func Execute(job, command, event string, concurrency int, verbose bool) error {
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

	err = JobCommand(tx, job, command, event, concurrency, verbose)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return data.NewDatabaseError(err)
	}

	if err = data.UpdateJournalModeDefault(db); err != nil {
		return err
	}

	return nil
}

func runAllocationCommand(job string, workerIndex int, workerIP string, tDisabled int, command, event string, verbose bool) error {
	workerDir := bucket.GetTempWorkerPath(workerIP)
	jobDir := path.Join(workerDir, "jobs", job)
	commandPath := fmt.Sprintf("%s.py", command)

	cmd := exec.Command("python3", commandPath)
	cmd.Dir = path.Join(jobDir, "_modules")

	envs := os.Environ()
	envs = append(envs, "ALLOCATION_INDEX="+strconv.Itoa(workerIndex))
	envs = append(envs, "ALLOCATION_IP="+workerIP)
	envs = append(envs, fmt.Sprintf("ALLOCATION_DISABLED=%d", tDisabled))
	envs = append(envs, "JOB="+job)
	envs = append(envs, "EVENT="+event)
	envs = append(envs, "DB_PATH="+bucket.GetDatabaseAbsPath())
	envs = append(envs, "COMMAND="+command)
	envs = append(envs, fmt.Sprintf("SESSION_EPOCH=%d", kv.GetKVStore().GlobalUnix))
	cmd.Env = envs

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stdout
	}

	return cmd.Run()
}
