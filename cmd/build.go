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
		purgeJobCommandKV, _ := cmd.Flags().GetBool("purge-job-kv")
		if err := build.Execute(build.Options{PurgeJobCommandKV: purgeJobCommandKV}); err != nil {
			fmt.Fprintln(os.Stderr, formatCommandError("build", err))
			os.Exit(1)
		}
	},
}

func init() {
	maandCmd.AddCommand(buildCmd)
	buildCmd.Flags().Bool(
		"purge-job-kv",
		false,
		"Mark vars/job/<job> and secrets/job/<job> deleted when a job has no active allocations",
	)
}
