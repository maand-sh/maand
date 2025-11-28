// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"

	"maand/initialize"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initializes the bucket",
	Run: func(cmd *cobra.Command, args []string) {
		err := initialize.Execute()
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(initCmd)
}
