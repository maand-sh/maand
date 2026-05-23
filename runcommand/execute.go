// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package runcommand runs shell commands on bucket workers via the maand container.
package runcommand

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"maand/bucket"
	"maand/data"
	"maand/healthcheck"
	"maand/prereq"
	"maand/utils"
	"maand/worker"

	"github.com/google/uuid"
)

const (
	defaultCommandFile = "command.sh"
	healthCheckDelay   = 10 * time.Second
)

// Execute runs a shell script on selected workers. The script comes from shellCommand
// or, when empty, from workspace/command.sh.
//
// Workers are processed in batches of batchSize (-c): at most batchSize workers run in
// parallel per batch. When runHealthChecks is true, all jobs are health-checked after
// each batch completes.
func Execute(workerCSV, labelCSV string, batchSize int, shellCommand string, runHealthChecks bool) error {
	if batchSize < 1 {
		return errConcurrencyTooLow()
	}
	if workerCSV != "" && labelCSV != "" {
		return errWorkersAndLabelsTogether()
	}

	db, err := data.OpenDatabase(true)
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

	targetWorkerIPs, err := resolveTargetWorkerIPs(tx, workerCSV, labelCSV)
	if err != nil {
		return err
	}
	if len(targetWorkerIPs) == 0 {
		return errNoTargetWorkers()
	}

	if err := prereq.CheckLocalRunCommand(); err != nil {
		return err
	}

	conf, err := bucket.GetMaandConf()
	if err != nil {
		return err
	}
	if err := worker.CheckRunCommandPrerequisites(targetWorkerIPs, conf.UseSUDO); err != nil {
		return err
	}

	scriptContent, err := loadShellScript(shellCommand)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(bucket.TempLocation, os.ModePerm); err != nil {
		return bucket.UnexpectedError(err)
	}

	rt, err := bucket.SetupRuntime(bucketID)
	if err != nil {
		return err
	}
	defer func() {
		_ = rt.Stop()
	}()

	var jobNames []string
	var cancelHealthCheck context.CancelFunc
	if runHealthChecks {
		jobNames, err = data.GetJobs(tx)
		if err != nil {
			return err
		}
		cancelHealthCheck, err = healthcheck.PrepareRuntime(tx)
		if err != nil {
			return err
		}
		defer cancelHealthCheck()
	}

	batchIterator := utils.NewStringIterator(targetWorkerIPs, batchSize)
	for batchNumber := 1; ; batchNumber++ {
		workerBatch, hasMore := batchIterator()
		if !hasMore {
			break
		}

		if err := runScriptOnWorkerBatch(rt, bucketID, scriptContent, workerBatch); err != nil {
			return err
		}

		if runHealthChecks {
			time.Sleep(healthCheckDelay)
			if err := healthcheck.RunJobs(tx, rt, true, true, jobNames); err != nil {
				return fmt.Errorf("after worker batch %d: %w", batchNumber, err)
			}
		}
	}

	return nil
}

func resolveTargetWorkerIPs(tx *sql.Tx, workerCSV, labelCSV string) ([]string, error) {
	workerFilter := parseCSVList(workerCSV)
	labelFilter := parseCSVList(labelCSV)

	switch {
	case len(workerFilter) > 0:
		bucketWorkerIPs, err := data.GetWorkers(tx, nil)
		if err != nil {
			return nil, err
		}

		unknownWorkers := utils.Difference(workerFilter, bucketWorkerIPs)
		if len(unknownWorkers) > 0 {
			return nil, errUnknownWorkers(unknownWorkers)
		}
		return utils.Unique(workerFilter), nil

	case len(labelFilter) > 0:
		workerIPs, err := data.GetWorkers(tx, labelFilter)
		if err != nil {
			return nil, err
		}
		return utils.Unique(workerIPs), nil

	default:
		workerIPs, err := data.GetWorkers(tx, nil)
		if err != nil {
			return nil, err
		}
		return utils.Unique(workerIPs), nil
	}
}

func parseCSVList(csv string) []string {
	if csv == "" {
		return nil
	}

	parts := strings.Split(csv, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func loadShellScript(shellCommand string) ([]byte, error) {
	if strings.TrimSpace(shellCommand) != "" {
		return []byte(shellCommand), nil
	}

	commandFilePath := path.Join(bucket.WorkspaceLocation, defaultCommandFile)
	if _, err := os.Stat(commandFilePath); err != nil {
		if os.IsNotExist(err) {
			return nil, errCommandRequired()
		}
		return nil, bucket.UnexpectedError(err)
	}

	content, err := os.ReadFile(commandFilePath)
	if err != nil {
		return nil, bucket.UnexpectedError(err)
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return nil, errEmptyCommand()
	}

	return content, nil
}

// runScriptOnWorkerBatch runs the script on every worker in the batch concurrently.
// The caller must keep the batch size at or below -c; batches are built by Execute.
func runScriptOnWorkerBatch(rt *bucket.Runtime, bucketID string, scriptContent []byte, workerBatch []string) error {
	workerBatch = utils.Unique(workerBatch)
	if len(workerBatch) == 0 {
		return nil
	}

	var waitGroup sync.WaitGroup
	var failureMu sync.Mutex
	failures := make(map[string]error, len(workerBatch))

	for _, workerIP := range workerBatch {
		waitGroup.Add(1)

		go func(targetWorkerIP string) {
			defer waitGroup.Done()

			commandFilePath, err := writeWorkerCommandScript(bucketID, targetWorkerIP, scriptContent)
			if err != nil {
				failureMu.Lock()
				failures[targetWorkerIP] = fmt.Errorf("create command script: %w", err)
				failureMu.Unlock()
				return
			}
			defer func() {
				_ = os.Remove(commandFilePath)
			}()

			execEnv := []string{
				fmt.Sprintf("BUCKET_ID=%s", bucketID),
				fmt.Sprintf("WORKER_IP=%s", targetWorkerIP),
			}

			if err := worker.ExecuteFileCommand(rt, targetWorkerIP, commandFilePath, execEnv); err != nil {
				failureMu.Lock()
				failures[targetWorkerIP] = err
				failureMu.Unlock()
			}
		}(workerIP)
	}

	waitGroup.Wait()
	return newRunCommandError(failures)
}

func writeWorkerCommandScript(bucketID, workerIP string, scriptContent []byte) (string, error) {
	// Unique name per invocation so concurrent workers never clobber the same script file.
	scriptFileName := fmt.Sprintf("command-%s-%s.sh", sanitizeWorkerIPForFilename(workerIP), uuid.NewString())
	scriptFilePath := path.Join(bucket.TempLocation, scriptFileName)

	prefix := strings.Join([]string{
		fmt.Sprintf("export BUCKET_ID=%s", bucketID),
		fmt.Sprintf("export WORKER_IP=%s", workerIP),
	}, "\n")

	fileContent := prefix + "\n" + string(scriptContent)
	if err := os.WriteFile(scriptFilePath, []byte(fileContent), 0o644); err != nil {
		return "", bucket.UnexpectedError(err)
	}

	return scriptFilePath, nil
}

func sanitizeWorkerIPForFilename(workerIP string) string {
	replacer := strings.NewReplacer(".", "_", ":", "_", "/", "_", " ", "_")
	return replacer.Replace(workerIP)
}

