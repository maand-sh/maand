package cmd

import (
	"github.com/spf13/cobra"
	"maand/data"
	"maand/health_check"
	"maand/utils"
	"os"
	"strings"
	"sync"
)

var healthCheckCmd = &cobra.Command{
	Use:   "health_check",
	Short: "Runs health check",
	Run: func(cmd *cobra.Command, args []string) {
		db, err := data.GetDatabase(true)
		utils.Check(err)

		tx, err := db.Begin()
		utils.Check(err)
		defer func() {
			_ = tx.Rollback()
		}()

		flags := cmd.Flags()
		wait, _ := flags.GetBool("wait")

		jobsStr, _ := flags.GetString("jobs")
		var jobsFilter []string
		if len(jobsStr) > 0 {
			jobsFilter = strings.Split(strings.Trim(jobsStr, ""), ",")
		}

		workers := data.GetWorkers(tx, nil)
		data.ValidateBucketUpdateSeq(tx, workers)

		var wg sync.WaitGroup
		jobs := data.GetJobs(tx)
		for _, job := range jobs {
			if len(jobsFilter) > 0 && len(utils.Intersection(jobsFilter, []string{job})) == 0 {
				continue
			}
			wg.Add(1)
			go func(tJob string) {
				defer wg.Done()
				hcErr := health_check.Execute(tx, wait, tJob)
				if hcErr != nil {
					err = hcErr
				}
			}(job)
		}
		wg.Wait()

		if err != nil {
			os.Exit(1)
		}

		err = tx.Commit()
		utils.Check(err)
	},
}

func init() {
	maandCmd.AddCommand(healthCheckCmd)
	healthCheckCmd.Flags().BoolP("wait", "", false, "")
	healthCheckCmd.Flags().StringP("jobs", "", "", "")
}
