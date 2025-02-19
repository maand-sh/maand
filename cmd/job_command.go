package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"maand/job_command"
	"maand/utils"
)

var jobCommandCmd = &cobra.Command{
	Use:   "job_command [job] [command]",
	Short: "Runs job command across allocations",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		verbose, _ := flags.GetBool("verbose")
		concurrency, _ := flags.GetInt("concurrency")
		if concurrency < 1 {
			utils.Check(fmt.Errorf("concurrency must be at least 1"))
		}

		job := args[0]
		command := args[1]

		err := job_command.Execute(job, command, "direct", concurrency, verbose)
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(jobCommandCmd)
	maandCmd.Flags().BoolP("verbose", "", false, "")
	maandCmd.Flags().IntP("concurrency", "", 1, "")
}
