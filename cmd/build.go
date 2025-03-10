// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"
	"maand/build"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build and plan object in the bucket",
	Run: func(cmd *cobra.Command, args []string) {
		err := build.Execute()
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(buildCmd)
}
