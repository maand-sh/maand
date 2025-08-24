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
	"runtime"
	"strings"
	"sync"
)

//go:embed maand.py
var MaandPy []byte

func JobCommand(tx *sql.Tx, dockerClient *bucket.DockerClient, job, command, event string, concurrency int, verbose bool, envs []string) error {
	allowedCommands, err := data.GetJobCommands(tx, job, event)
	if err != nil {
		return err
	}

	if len(utils.Intersection([]string{command}, allowedCommands)) == 0 {
		return &JobCommandNotFoundError{Command: command, Job: job, Event: event}
	}

	workers, err := data.GetActiveAllocations(tx, job)
	if err != nil {
		return err
	}

	kvStore := kv.GetKVStore()

	getCerts := func(workerIP string) error {
		workerDir := bucket.GetTempWorkerPath(workerIP)

		keys, err := kvStore.GetKeys(tx, fmt.Sprintf("maand/job/%s/worker/%s", job, workerIP))
		if err != nil {
			return err
		}

		for _, key := range keys {
			if !strings.HasPrefix(key, "certs/") {
				continue
			}

			content, err := kvStore.Get(tx, fmt.Sprintf("maand/job/%s/worker/%s", job, workerIP), key)
			if err != nil {
				return err
			}

			err = os.WriteFile(path.Join(workerDir, "jobs", job, "_modules", key), []byte(content), os.ModePerm)
			if err != nil {
				return err
			}
		}

		caPath := path.Join(workerDir, "jobs", job, "_modules", "certs/ca.crt")
		if _, err := os.Stat(caPath); os.IsNotExist(err) {
			content, err := kvStore.Get(tx, fmt.Sprintf("maand/worker"), "certs/ca.crt")
			if err != nil {
				return err
			}

			err = os.WriteFile(caPath, []byte(content), os.ModePerm)
			if err != nil {
				return err
			}
		}

		return nil
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
		err = os.WriteFile(path.Join(moduleDir, "maand.py"), MaandPy, os.ModePerm)
		if err != nil {
			return err
		}

		certsDir := path.Join(workerDir, "jobs", job, "_modules", "certs")
		err = os.MkdirAll(certsDir, os.ModePerm)
		if err != nil {
			return err
		}

		err = getCerts(workerIP)
		if err != nil {
			return err
		}
	}

	allocErrors := map[string]error{}
	var semaphore = make(chan struct{}, concurrency)
	var wait sync.WaitGroup
	var mu sync.Mutex

	for _, workerIP := range workers {
		wait.Add(1)
		semaphore <- struct{}{}

		disabled, err := data.IsAllocationDisabled(tx, workerIP, job)
		if err != nil {
			return err
		}

		allocID, err := data.GetAllocationID(tx, workerIP, job)
		if err != nil {
			return err
		}

		go func(tAllocID string, tWorkerIP string, tDisabled int) {
			defer wait.Done()
			defer func() { <-semaphore }()

			err := runAllocationCommand(dockerClient, tAllocID, job, tWorkerIP, tDisabled, command, event, verbose, envs)
			if err != nil {
				mu.Lock()
				allocErrors[tWorkerIP] = err
				mu.Unlock()
			}
		}(allocID, workerIP, disabled)
	}

	wait.Wait()
	close(semaphore)

	if len(allocErrors) > 0 {
		return &JobCommandError{Job: job, Command: command, Err: allocErrors}
	}

	return nil
}

func Execute(job, command, event string, concurrency int, verbose bool, envs []string) error {
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

	cancel := SetupServer(tx)
	defer cancel()

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

	err = os.MkdirAll(bucket.TempLocation, os.ModePerm)
	if err != nil {
		return err
	}

	for _, workerIP := range workers {
		err = worker.KeyScan(dockerClient, workerIP)
		if err != nil {
			return err
		}
	}

	err = JobCommand(tx, dockerClient, job, command, event, concurrency, verbose, envs)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return data.NewDatabaseError(err)
	}

	return nil
}

func runAllocationCommand(dockerClient *bucket.DockerClient, allocID string, job string, workerIP string, disabled int, command, event string, verbose bool, pEnvs []string) error {
	workerDir := path.Join("/bucket", "tmp", "workers", workerIP)
	jobDir := path.Join(workerDir, "jobs", job)
	commandPath := fmt.Sprintf("%s.py", command)

	cmd := exec.Command("python3", commandPath)
	cmd.Dir = path.Join(jobDir, "_modules")
	// TODO: copy job certs to modules folder

	envs := os.Environ()
	envs = append(envs, pEnvs...)
	envs = append(envs, fmt.Sprintf("ALLOCATION_ID=%s", allocID))
	envs = append(envs, fmt.Sprintf("ALLOCATION_IP=%s", workerIP))
	envs = append(envs, fmt.Sprintf("DISABLED=%d", disabled))
	envs = append(envs, fmt.Sprintf("JOB=%s", job))
	envs = append(envs, fmt.Sprintf("EVENT=%s", event))
	envs = append(envs, fmt.Sprintf("COMMAND=%s", command))
	envs = append(envs, fmt.Sprintf("CONTAINER_HOST=%s", getContainerHost()))
	cmd.Env = envs

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stdout
	}

	currPath := fmt.Sprintf("cd %s/jobs/%s/_modules", workerDir, job)
	err := dockerClient.Exec(workerIP, []string{currPath, "python3 " + path.Join(jobDir, "_modules", commandPath)}, envs, verbose)
	if err != nil {
		return err
	}

	return nil
}

func getContainerHost() string {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return "host.docker.internal"
	}
	return "0.0.0.0"
}
