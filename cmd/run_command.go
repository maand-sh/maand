// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"maand/run_command"
)

var runCommandCmd = &cobra.Command{
	Use:   "run_command",
	Short: "Runs shell commands across workers",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		workerStr, _ := flags.GetString("workers")
		labelStr, _ := flags.GetString("labels")
		concurrency, _ := flags.GetInt("concurrency")
		shCommand, _ := flags.GetString("cmd")
		disableCheck, _ := flags.GetBool("disable-bucket-update-check")
		healthCheck, _ := flags.GetBool("health_check")

		err := run_command.Execute(workerStr, labelStr, concurrency, shCommand, disableCheck, healthCheck)
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(runCommandCmd)
	runCommandCmd.Flags().StringP("cmd", "", "", "inline command")
	runCommandCmd.Flags().StringP("workers", "w", "", "comma seperated workers")
	runCommandCmd.Flags().StringP("labels", "l", "", "comma seperated labels")
	runCommandCmd.Flags().IntP("concurrency", "c", 2, "concurrency")
	runCommandCmd.Flags().BoolP("disable-bucket-update-check", "d", false, "disable cluster check")
	runCommandCmd.Flags().BoolP("health_check", "", false, "runs health check")
	runCommandCmd.Flags().BoolP("local", "", false, "runs local")
}
