package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"maand/data"
	"maand/job_command"
	"maand/utils"
	"maand/worker"
)

var jobCommandCmd = &cobra.Command{
	Use:   "job_command [job] [command]",
	Short: "Runs job command across allocations",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		db, err := data.GetDatabase(true)
		utils.Check(err)

		tx, err := db.Begin()
		utils.Check(err)
		defer func() {
			_ = tx.Rollback()
		}()

		job := args[0]
		command := args[1]

		flags := cmd.Flags()
		concurrency, _ := flags.GetInt("concurrency")
		if concurrency < 1 {
			utils.Check(fmt.Errorf("concurrency must be at least 1"))
		}

		workers := data.GetWorkers(tx, nil)
		for _, workerIP := range workers {
			worker.KeyScan(workerIP)
		}

		data.ValidateBucketUpdateSeq(tx, workers)

		err = job_command.Execute(tx, job, command, "direct", concurrency)
		utils.Check(err)

		err = tx.Commit()
		utils.Check(err)
	},
}

func init() {
	maandCmd.AddCommand(jobCommandCmd)
}
