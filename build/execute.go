// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package build provides interfaces to build workspace
package build

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"maand/bucket"
	"maand/data"
	"maand/jobcommand"
	"maand/kv"
	"maand/workspace"
)

const postBuildEvent = "post_build"

// Options configures optional build behavior.
type Options struct {
	// PurgeJobCommandKV marks vars/job/<job> and secrets/job/<job> deleted when a job
	// has no active allocations (or is removed from the workspace).
	PurgeJobCommandKV bool
}

func runPostBuildHooks(tx *sql.Tx) error {
	if err := kv.Initialize(tx); err != nil {
		return fmt.Errorf("post_build: init kv: %w", err)
	}

	cancelJobCommandServer := jobcommand.StartRuntimeAPI(tx)
	defer cancelJobCommandServer()

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return fmt.Errorf("post_build: get bucket id: %w", err)
	}

	rt, err := bucket.SetupRuntime(bucketID)
	if err != nil {
		return fmt.Errorf("post_build: setup runtime: %w", err)
	}
	defer func() {
		_ = rt.Stop()
	}()

	maxSequence, err := data.GetMaxDeploymentSeq(tx)
	if err != nil {
		return fmt.Errorf("post_build: get max deployment sequence: %w", err)
	}

	var hookErrors []error
	for sequence := 0; sequence <= maxSequence; sequence++ {
		jobsAtSequence, err := data.GetJobsByDeploymentSeq(tx, sequence)
		if err != nil {
			return fmt.Errorf("post_build: get jobs for deployment_seq %d: %w", sequence, err)
		}

		for _, jobName := range jobsAtSequence {
			commandNames, err := data.GetJobCommands(tx, jobName, postBuildEvent)
			if err != nil {
				hookErrors = append(hookErrors, fmt.Errorf("post_build: get commands for job %s: %w", jobName, err))
				continue
			}

			for _, commandName := range commandNames {
				err := jobcommand.JobCommand(tx, rt, jobName, commandName, postBuildEvent, 1, true, nil)
				if err != nil {
					hookErrors = append(hookErrors, fmt.Errorf("post_build: job %s command %s: %w", jobName, commandName, err))
				}
			}
		}
	}

	if err := errors.Join(hookErrors...); err != nil {
		return err
	}

	if err := kv.PersistToSessionTransaction(tx); err != nil {
		return fmt.Errorf("post_build: persist kv: %w", err)
	}
	return nil
}

func Execute(opts ...Options) error {
	var options Options
	if len(opts) > 0 {
		options = opts[0]
	}

	db, err := data.OpenDatabase(true)
	if err != nil {
		return err
	}

	defer func() {
		_ = db.Close()
		_ = os.RemoveAll(bucket.TempLocation)
	}()

	buildTx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = buildTx.Rollback()
	}()

	jobWorkspace := workspace.GetWorkspace()

	if err := kv.Initialize(buildTx); err != nil {
		return err
	}

	removedWorkers, err := BuildWorkers(buildTx, jobWorkspace)
	if err != nil {
		return err
	}

	removedJobs, err := BuildJobs(buildTx, jobWorkspace)
	if err != nil {
		return err
	}

	workspaceJobNames, err := jobWorkspace.GetJobs()
	if err != nil {
		return err
	}
	if err := ValidateJobCommandDemands(jobWorkspace, workspaceJobNames); err != nil {
		return err
	}

	if err := BuildAllocations(buildTx, jobWorkspace); err != nil {
		return err
	}

	if err := BuildDeploymentSequence(buildTx); err != nil {
		return err
	}

	if err := BuildVariables(buildTx, jobWorkspace, removedWorkers, removedJobs, options.PurgeJobCommandKV); err != nil {
		return err
	}

	if err := BuildCerts(buildTx); err != nil {
		return err
	}

	if err := BuildJobAllocationVariables(buildTx, removedJobs); err != nil {
		return err
	}

	if err := os.MkdirAll(bucket.TempLocation, 0o755); err != nil {
		return bucket.UnexpectedError(err)
	}

	if err := kv.GetStore().PurgeStaleVersions(buildTx, 7); err != nil {
		return err
	}

	if err := ValidateWorkerResources(buildTx); err != nil {
		return err
	}

	if err := kv.PersistToTransaction(buildTx, kv.GetStore()); err != nil {
		return err
	}

	if err := buildTx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}

	if _, err := db.Exec("VACUUM"); err != nil {
		return bucket.DatabaseError(err)
	}

	postBuildTx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = postBuildTx.Rollback()
	}()

	if err := runPostBuildHooks(postBuildTx); err != nil {
		return err
	}

	if err := postBuildTx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}

	return nil
}
