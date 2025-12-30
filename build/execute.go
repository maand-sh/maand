// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package build provides interfaces to build workspace
package build

import (
	"database/sql"
	"fmt"
	"os"

	"maand/bucket"
	"maand/data"
	"maand/jobcommand"
	"maand/kv"
	"maand/workspace"
)

func runPostBuild(tx *sql.Tx) error {
	cancel := jobcommand.SetupServer(tx)
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

	maxDeploymentSequence, err := data.GetMaxDeploymentSeq(tx)
	if err != nil {
		return err
	}

	for deploymentSeq := 0; deploymentSeq <= maxDeploymentSequence; deploymentSeq++ {
		jobs, err := data.GetJobsByDeploymentSeq(tx, deploymentSeq)
		if err != nil {
			return err
		}

		for _, job := range jobs {
			postBuildCommands, err := data.GetJobCommands(tx, job, "post_build")
			if err != nil {
				return err
			}

			if len(postBuildCommands) == 0 {
				continue
			}
			for _, command := range postBuildCommands {
				err := jobcommand.JobCommand(tx, dockerClient, job, command, "post_build", 1, true, []string{})
				if err != nil {
					return fmt.Errorf("post_build failed: %v", err)
				}
			}
		}
	}
	return nil
}

func Execute() error {
	db, err := data.GetDatabase(true)
	if err != nil {
		return err
	}

	defer func() {
		_ = db.Close()
		_ = os.RemoveAll(bucket.TempLocation)
	}()

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	ws := workspace.GetWorkspace()

	err = kv.Initialize(tx)
	if err != nil {
		return err
	}

	err = Workers(tx, ws)
	if err != nil {
		return err
	}

	err = Jobs(tx, ws)
	if err != nil {
		return err
	}

	err = Allocations(tx, ws)
	if err != nil {
		return err
	}

	err = DeploymentSequence(tx)
	if err != nil {
		return err
	}

	err = Variables(tx)
	if err != nil {
		return err
	}

	err = Certs(tx)
	if err != nil {
		return err
	}

	err = Container(tx)
	if err != nil {
		return err
	}

	err = kv.GetKVStore().GC(tx, 7)
	if err != nil {
		return err
	}

	err = Validate(tx)
	if err != nil {
		return err
	}

	err = kv.SaveKeyValueStore(tx, kv.GetKVStore())
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return bucket.DatabaseError(err)
	}

	_, err = db.Exec("VACUUM")
	if err != nil {
		return bucket.DatabaseError(err)
	}

	tx, err = db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	err = runPostBuild(tx)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}

	return nil
}
