// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"log"
	"os"
	"strings"

	"maand/bucket"
	"maand/runcommand"

	"github.com/spf13/cobra"
)

var runCommandCmd = &cobra.Command{
	Use:   "run_command [command]",
	Short: "Run a shell command across workers",
	Long: `Run a shell command on workers in the bucket.

Workers are executed in batches. Use -c to set how many workers run in parallel per batch
(for example, -c 3 with 10 workers runs three batches: 3, then 3, then 4).

With --health_check, every job is health-checked after each batch finishes.

Provide the command as an argument, or leave it empty to use workspace/command.sh.

Examples:
  maand run_command "uptime"
  maand run_command -c 3 "hostname"
  maand run_command -w 10.0.0.1,10.0.0.2 -c 2 "uptime"
  maand run_command -l worker --health_check -c 2 "df -h"`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()

		workerCSV, _ := flags.GetString("workers")
		labelCSV, _ := flags.GetString("labels")
		batchSize, _ := flags.GetInt("concurrency")
		runHealthChecks, _ := flags.GetBool("health_check")

		shellCommand := ""
		if len(args) > 0 {
			shellCommand = strings.TrimSpace(args[0])
		}

		err := runcommand.Execute(workerCSV, labelCSV, batchSize, shellCommand, runHealthChecks)
		if errors.Is(err, bucket.ErrRunCommand) {
			log.Println(err)
			os.Exit(1)
		}
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(runCommandCmd)
	runCommandCmd.Flags().StringP("workers", "w", "", "Comma-separated worker IPs")
	runCommandCmd.Flags().StringP("labels", "l", "", "Comma-separated worker labels")
	runCommandCmd.Flags().IntP("concurrency", "c", 1, "Workers per batch (parallel executions at a time)")
	runCommandCmd.Flags().Bool("health_check", false, "Run health checks for all jobs after every batch")
}
