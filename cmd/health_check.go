// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"

	"maand/healthcheck"

	"github.com/spf13/cobra"
)

var healthCheckCmd = &cobra.Command{
	Use:   "health_check",
	Short: "Runs health check",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		wait, _ := flags.GetBool("wait")
		jobsComma, _ := flags.GetString("jobs")
		verbose, _ := flags.GetBool("verbose")
		err := healthcheck.Execute(wait, verbose, jobsComma)
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(healthCheckCmd)
	healthCheckCmd.Flags().BoolP("verbose", "", false, "")
	healthCheckCmd.Flags().BoolP("wait", "", false, "")
	healthCheckCmd.Flags().StringP("jobs", "", "", "")
}
