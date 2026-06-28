// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"

	"maand/jobcommand"

	"github.com/spf13/cobra"
)

var jobCommandCmd = &cobra.Command{
	Use:     "jobcommand <command> [job]",
	Aliases: []string{"job_command"},
	Short:   "Run a manifest job command across allocations",
	Long: `Run a job command registered for the cli event.

When job is omitted, the command runs on every job in the catalog that defines it.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		verbose, _ := flags.GetBool("verbose")

		concurrency, _ := flags.GetInt("concurrency")
		if concurrency < 1 {
			log.Fatal("concurrency must be at least 1")
		}

		command := args[0]
		job := ""
		if len(args) > 1 {
			job = args[1]
		}

		err := jobcommand.Execute(command, job, "cli", concurrency, verbose, []string{})
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
