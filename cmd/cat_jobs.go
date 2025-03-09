// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"
	"maand/cat"

	"github.com/spf13/cobra"
)

var catJobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "Shows available jobs",
	Run: func(cmd *cobra.Command, args []string) {
		err := cat.Jobs()
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	catCmd.AddCommand(catJobsCmd)
}
