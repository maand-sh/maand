// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"log"

	"maand/jobcommand"

	"github.com/spf13/cobra"
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
			fmt.Println("concurrency must be at least 1")
		}

		job := args[0]
		command := args[1]

		err := jobcommand.Execute(job, command, "cli", concurrency, verbose, []string{})
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(jobCommandCmd)
	jobCommandCmd.Flags().BoolP("verbose", "", false, "")
	jobCommandCmd.Flags().IntP("concurrency", "", 1, "")
}
