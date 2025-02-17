package cmd

import (
	"github.com/spf13/cobra"
	"maand/job_control"
)

var jobRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs job target, ex: start, stop and restarts",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("workers")
		jobsComma, _ := flags.GetString("jobs")
		healthCheck, _ := flags.GetBool("health_check")
		job_control.Execute(jobsComma, workersComma, args[0], healthCheck)
	},
}

var jobStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Runs job target start",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("workers")
		jobsComma, _ := flags.GetString("jobs")
		healthCheck, _ := flags.GetBool("health_check")
		job_control.Execute(jobsComma, workersComma, "start", healthCheck)
	},
}

var jobStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Runs job target stop",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("workers")
		jobsComma, _ := flags.GetString("jobs")
		healthCheck, _ := flags.GetBool("health_check")
		job_control.Execute(jobsComma, workersComma, "stop", healthCheck)
	},
}

var jobRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Runs job target restart",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("workers")
		jobsComma, _ := flags.GetString("jobs")
		healthCheck, _ := flags.GetBool("health_check")
		job_control.Execute(jobsComma, workersComma, "restart", healthCheck)
	},
}

var jobStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Runs job target status",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("workers")
		jobsComma, _ := flags.GetString("jobs")
		healthCheck, _ := flags.GetBool("health_check")
		job_control.Execute(jobsComma, workersComma, "restart", healthCheck)
	},
}

func init() {
	jobCmd.AddCommand(jobRunCmd)
	jobCmd.AddCommand(jobStartCmd)
	jobCmd.AddCommand(jobStopCmd)
	jobCmd.AddCommand(jobRestartCmd)
	jobCmd.AddCommand(jobStatusCmd)

	for _, cmd := range []*cobra.Command{jobRunCmd, jobStartCmd, jobStopCmd, jobRestartCmd, jobStatusCmd} {
		cmd.Flags().String("workers", "", "comma separated workers")
		cmd.Flags().String("jobs", "", "comma separated jobs")
	}

	for _, cmd := range []*cobra.Command{jobRunCmd, jobStartCmd, jobRestartCmd} {
		cmd.Flags().String("health_check", "", "adds health check")
	}
}
