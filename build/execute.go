package build

import (
	"maand/bucket"
	"maand/data"
	"maand/job_command"
	"maand/utils"
	"maand/workspace"
	"os"
)

func Execute() {
	db, err := data.GetDatabase(true)
	utils.Check(err)
	defer func() {
		_ = db.Close()
		_ = os.RemoveAll(bucket.TempLocation)
	}()

	tx, err := db.Begin()
	utils.Check(err)
	defer func() {
		_ = tx.Rollback()
	}()

	ws := workspace.GetWorkspace()

	Workers(tx, ws)
	Jobs(tx, ws)
	Allocations(tx, ws)
	DeploymentSequence(tx)
	Variables(tx)
	Certs(tx)

	err = utils.GetKVStore().GC(tx, 7)
	utils.Check(err)

	// resource validation
	err = tx.Commit()
	utils.Check(err)

	_, _ = db.Exec("VACUUM")

	tx, err = db.Begin()
	utils.Check(err)
	defer func() {
		_ = tx.Rollback()
	}()

	maxDeploymentSequence := data.GetMaxDeploymentSeq(tx)
	for deploymentSeq := 0; deploymentSeq <= maxDeploymentSequence; deploymentSeq++ {
		jobs := data.GetJobsByDeploymentSeq(tx, deploymentSeq)
		for _, job := range jobs {
			postBuildCommands := data.GetJobCommands(tx, job, "post_build")
			if len(postBuildCommands) == 0 {
				continue
			}
			for _, command := range postBuildCommands {
				err = job_command.Execute(tx, job, command, "post_build", 1)
				utils.Check(err)
			}
		}
	}

	err = tx.Commit()
	utils.Check(err)

	_ = utils.ExecuteCommand([]string{"sync"})
}
