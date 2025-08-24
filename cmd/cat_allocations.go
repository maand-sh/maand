// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"maand/cat"
)

var catAllocationsCmd = &cobra.Command{
	Use:   "allocations",
	Short: "Shows available allocations",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		jobsStr, _ := flags.GetString("jobs")
		workersStr, _ := flags.GetString("workers")

		err := cat.Allocations(jobsStr, workersStr)
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	catCmd.AddCommand(catAllocationsCmd)
	catAllocationsCmd.Flags().String("workers", "", "comma separated workers")
	catAllocationsCmd.Flags().String("jobs", "", "comma separated jobs")
}
