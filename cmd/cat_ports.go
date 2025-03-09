// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"
	"maand/cat"

	"github.com/spf13/cobra"
)

var catJobPortsCmd = &cobra.Command{
	Use:   "job_ports",
	Short: "Shows available job ports",
	Run: func(cmd *cobra.Command, args []string) {
		err := cat.JobPorts()
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	catCmd.AddCommand(catJobPortsCmd)
}
