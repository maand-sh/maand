// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"

	"maand/jobcontrol"

	"github.com/spf13/cobra"
)

var jobRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs job target, ex: start, stop and restarts",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("allocations")
		target, _ := flags.GetString("target")
		healthCheck, _ := flags.GetBool("health_check")
		err := jobcontrol.Execute(args[0], workersComma, target, healthCheck)
		if err != nil {
			log.Fatalln(err)
		}
	},
}

var jobStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Runs job target start",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("allocations")
		healthCheck, _ := flags.GetBool("health_check")
		err := jobcontrol.Execute(args[0], workersComma, "start", healthCheck)
		if err != nil {
			log.Fatalln(err)
		}
	},
}

var jobStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Runs job target stop",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("allocations")
		healthCheck, _ := flags.GetBool("health_check")
		err := jobcontrol.Execute(args[0], workersComma, "stop", healthCheck)
		if err != nil {
			log.Fatal(err)
		}
	},
}

var jobRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Runs job target restart",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("allocations")
		healthCheck, _ := flags.GetBool("health_check")
		err := jobcontrol.Execute(args[0], workersComma, "restart", healthCheck)
		if err != nil {
			log.Fatal(err)
		}
	},
}

var jobStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Runs job target status",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workersComma, _ := flags.GetString("allocations")
		err := jobcontrol.Execute(args[0], workersComma, "status", false)
		if err != nil {
			log.Fatal(err)
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
		cmd.Flags().String("allocations", "", "comma separated allocations")
		cmd.Flags().Bool("health_check", false, "adds health check")
	}
	jobRunCmd.Flags().String("target", "", "")
	_ = jobRunCmd.MarkFlagRequired("target")
}
