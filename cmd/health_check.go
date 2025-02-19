package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"maand/health_check"
)

var healthCheckCmd = &cobra.Command{
	Use:   "health_check",
	Short: "Runs health check",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		wait, _ := flags.GetBool("wait")
		jobsComma, _ := flags.GetString("jobs")
		verbose, _ := flags.GetBool("verbose")
		err := health_check.Execute(wait, verbose, jobsComma)
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(healthCheckCmd)
	healthCheckCmd.Flags().BoolP("verbose", "", false, "")
	healthCheckCmd.Flags().BoolP("wait", "", false, "")
	healthCheckCmd.Flags().StringP("jobs", "", "", "")
}
