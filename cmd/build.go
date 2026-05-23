// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"maand/build"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Plan and build objects in the bucket",
	Run: func(cmd *cobra.Command, args []string) {
		if err := build.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, formatCommandError("build", err))
			os.Exit(1)
		}
	},
}

func init() {
	maandCmd.AddCommand(buildCmd)
}
