// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package health_check

import (
	"database/sql"
	"fmt"
	"maand/data"
	"maand/job_command"
	"maand/utils"
	"strings"
	"sync"
	"time"
)

func HealthCheck(tx *sql.Tx, wait bool, job string, verbose bool) error {
	commands, err := data.GetJobCommands(tx, job, "health_check")
	if err != nil {
		return err
	}

	if len(commands) == 0 {
		fmt.Printf("health check is undefined, job %s\n", job)
		return nil
	}

	healthCheckFunc := func() error {
		for _, cmd := range commands {
			err := job_command.JobCommand(tx, job, cmd, "health_check", 1, verbose)
			if err != nil {
				return err
			}
		}
		return nil
	}

	if wait {
		for i := 0; i < 30; i++ {
			time.Sleep(1 * time.Second)
			err := healthCheckFunc()
			if err == nil {
				fmt.Printf("health check is passed, job %s\n", job)
				return nil
			}
			fmt.Printf("health check is failed, job %s, retrying...\n", job)
		}
		return &HealthCheckError{Err: err, Job: job}
	} else {
		err := healthCheckFunc()
		if err != nil {
			fmt.Printf("health check is failed, job %s\n", job)
			return &HealthCheckError{Err: err, Job: job}
		}
		fmt.Printf("health check is passed, job %s\n", job)
	}
	return nil
}

func Execute(wait bool, verbose bool, jobsComma string) error {
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

	var jobsFilter []string
	if len(jobsComma) > 0 {
		jobsFilter = strings.Split(strings.Trim(jobsComma, ""), ",")
	}

	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}

	err = data.ValidateBucketUpdateSeq(tx, workers)
	if err != nil {
		return err
	}

	jobs, err := data.GetJobs(tx)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errJobs = make(map[string]error)
	for _, job := range jobs {
		if len(jobsFilter) > 0 {
			if len(utils.Intersection(jobsFilter, []string{job})) == 0 {
				continue
			}
		}
		wg.Add(1)

		go func(tJob string) {
			defer wg.Done()
			hcErr := HealthCheck(tx, wait, tJob, verbose)
			if hcErr != nil {
				mu.Lock()
				errJobs[tJob] = hcErr
				mu.Unlock()
			}
		}(job)
	}
	wg.Wait()

	if err = tx.Commit(); err != nil {
		return data.NewDatabaseError(err)
	}

	//TODO: deal with errors
	//if len(errJobs) > 0 {
	//	return fmt.Errorf("%v", errJobs)
	//}

	err = data.UpdateJournalModeDefault(db)
	if err != nil {
		return err
	}

	return nil
}
