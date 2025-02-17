package job_command

import (
	"database/sql"
	_ "embed"
	"fmt"
	"maand/bucket"
	"maand/data"
	"maand/utils"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
)

//go:embed maand.py
var MaandPy []byte

func Execute(tx *sql.Tx, job, command, event string, concurrency int) error {
	allowedCommands := data.GetJobCommands(tx, job, event)
	if len(utils.Intersection([]string{command}, allowedCommands)) == 0 {
		return fmt.Errorf("job command %s, event %s is not found. job %s", command, event, job)
	}

	workers := data.GetActiveAllocations(tx, job)
	for _, workerIP := range workers {
		workerDir := bucket.GetTempWorkerPath(workerIP)
		err := os.MkdirAll(workerDir, os.ModePerm)
		utils.Check(err)
		data.CopyJobFiles(tx, job, path.Join(workerDir, "jobs"))
		moduleDir := path.Join(workerDir, "jobs", job, "_modules")
		if _, err = os.Stat(moduleDir); err == nil {
			err = os.WriteFile(path.Join(moduleDir, "maand.py"), MaandPy, os.ModePerm)
			utils.Check(err)
		}
	}

	if concurrency <= 0 || strings.HasPrefix("command_parallel_", command) {
		concurrency = len(workers)
	}

	semaphore := make(chan struct{}, concurrency)
	var allocErrors []error

	var wait sync.WaitGroup
	var mu sync.Mutex

	for workerIndex, workerIP := range workers {
		wait.Add(1)
		semaphore <- struct{}{}
		disabled := data.IsAllocationDisabled(tx, workerIP, job)
		go func(tWorkerIndex int, tWorkerIP string, tDisabled int) {
			defer wait.Done()
			defer func() { <-semaphore }()

			err := runAllocationCommand(job, tWorkerIndex, tWorkerIP, tDisabled, command, event)
			if err != nil {
				mu.Lock()
				allocErrors = append(allocErrors, err)
				mu.Unlock()
			}
		}(workerIndex, workerIP, disabled)
	}

	_ = utils.ExecuteCommand([]string{"sync"})

	wait.Wait()
	close(semaphore)

	if len(allocErrors) > 0 {
		return fmt.Errorf("job %s, command %s failed", job, command)
	}
	return nil
}

func runAllocationCommand(job string, workerIndex int, workerIP string, tDisabled int, command, event string) error {
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
	envs = append(envs, fmt.Sprintf("SESSION_EPOCH=%d", utils.GetKVStore().GlobalUnix))
	cmd.Env = envs

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	return cmd.Run()
}
