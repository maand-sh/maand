// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"
	"maand/cat"

	"github.com/spf13/cobra"
)

var catCertsCmd = &cobra.Command{
	Use:   "certs",
	Short: "List TLS certificates and expiration dates",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		jobsStr, _ := flags.GetString("jobs")
		workersStr, _ := flags.GetString("workers")

		if err := cat.Certs(jobsStr, workersStr); err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	catCmd.AddCommand(catCertsCmd)
	catCertsCmd.Flags().String("jobs", "", "Comma-separated job names")
	catCertsCmd.Flags().String("workers", "", "Comma-separated worker IPs")
}
