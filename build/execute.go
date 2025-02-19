package build

import (
	"database/sql"
	"maand/bucket"
	"maand/data"
	"maand/job_command"
	"maand/kv"
	"maand/utils"
	"maand/workspace"
	"os"
)

func runPostBuild(tx *sql.Tx) error {
	maxDeploymentSequence, err := data.GetMaxDeploymentSeq(tx)
	if err != nil {
		return err
	}

	for deploymentSeq := 0; deploymentSeq <= maxDeploymentSequence; deploymentSeq++ {
		jobs := data.GetJobsByDeploymentSeq(tx, deploymentSeq)
		for _, job := range jobs {
			postBuildCommands, err := data.GetJobCommands(tx, job, "post_build")
			if err != nil {
				return err
			}

			if len(postBuildCommands) == 0 {
				continue
			}
			for _, command := range postBuildCommands {
				err := job_command.JobCommand(tx, job, command, "post_build", 1, true)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func Execute() error {
	db, err := data.GetDatabase(true)
	if err != nil {
		return data.NewDatabaseError(err)
	}
	defer func() {
		_ = db.Close()
		_ = os.RemoveAll(bucket.TempLocation)
	}()

	tx, err := db.Begin()
	if err != nil {
		return data.NewDatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	ws := workspace.GetWorkspace()

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

	// TODO: resource validation

	err = kv.GetKVStore().GC(tx, 7)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return data.NewDatabaseError(err)
	}

	_, err = db.Exec("VACUUM")
	if err != nil {
		return data.NewDatabaseError(err)
	}

	tx, err = db.Begin()
	if err != nil {
		return data.NewDatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	err = runPostBuild(tx)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return data.NewDatabaseError(err)
	}

	if err = data.UpdateJournalModeDefault(db); err != nil {
		return err
	}

	err = utils.ExecuteCommand([]string{"sync"})
	if err != nil {
		return err
	}

	return nil
}
