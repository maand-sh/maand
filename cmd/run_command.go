// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"log"
	"os"

	"maand/bucket"
	"maand/runcommand"

	"github.com/spf13/cobra"
)

var runCommandCmd = &cobra.Command{
	Use:   "run_command",
	Short: "Runs shell commands across workers",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workerStr, _ := flags.GetString("workers")
		labelStr, _ := flags.GetString("labels")
		concurrency, _ := flags.GetInt("concurrency")
		shCommand := ""
		if len(args) > 0 {
			shCommand = args[0]
		}
		healthCheck, _ := flags.GetBool("health_check")

		err := runcommand.Execute(workerStr, labelStr, concurrency, shCommand, healthCheck)
		if errors.Is(err, bucket.ErrRunCommand) {
			os.Exit(1)
		}
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(runCommandCmd)
	runCommandCmd.Flags().StringP("workers", "w", "", "comma seperated workers")
	runCommandCmd.Flags().StringP("labels", "l", "", "comma seperated labels")
	runCommandCmd.Flags().IntP("concurrency", "c", 1, "concurrency")
	runCommandCmd.Flags().BoolP("health_check", "", false, "runs health check")
	runCommandCmd.Flags().BoolP("local", "", false, "runs local")
}
