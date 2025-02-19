package cmd

import (
	"fmt"
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
		target, _ := flags.GetString("target")
		healthCheck, _ := flags.GetBool("health_check")
		err := job_control.Execute(args[0], workersComma, target, healthCheck)
		if err != nil {
			fmt.Println(err)
		}
	},
}

var jobStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Runs job target start",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("workers")
		healthCheck, _ := flags.GetBool("health_check")
		err := job_control.Execute(args[0], workersComma, "start", healthCheck)
		if err != nil {
			fmt.Println(err)
		}
	},
}

var jobStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Runs job target stop",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("workers")
		healthCheck, _ := flags.GetBool("health_check")
		err := job_control.Execute(args[0], workersComma, "stop", healthCheck)
		if err != nil {
			fmt.Println(err)
		}
	},
}

var jobRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Runs job target restart",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("workers")
		healthCheck, _ := flags.GetBool("health_check")
		err := job_control.Execute(args[0], workersComma, "restart", healthCheck)
		if err != nil {
			fmt.Println(err)
		}
	},
}

var jobStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Runs job target status",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("workers")
		err := job_control.Execute(args[0], workersComma, "status", false)
		if err != nil {
			fmt.Println(err)
		}
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
		cmd.Flags().String("health_check", "", "adds health check")
	}
	jobRunCmd.Flags().String("target", "", "")
	_ = jobRunCmd.MarkFlagRequired("target")
}
