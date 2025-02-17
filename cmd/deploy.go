package cmd

import (
	"github.com/spf13/cobra"
	"maand/build"
	"maand/deploy"
	"strings"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy bucket to workers",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		jobsStr, _ := flags.GetString("jobs")
		var jobsFilter []string
		if len(jobsStr) > 0 {
			jobsFilter = strings.Split(strings.Trim(jobsStr, ""), ",")
		}
		buildFlag, _ := flags.GetBool("build")
		if buildFlag {
			build.Execute()
		}
		deploy.Execute(jobsFilter)
	},
}

func init() {
	maandCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringP("jobs", "", "", "comma seperated jobs")
	deployCmd.Flags().BoolP("build", "b", false, "build before deploy")
}
