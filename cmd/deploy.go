// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"maand/build"
	"maand/deploy"
	"strings"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy bucket to workers",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		jobsStr, _ := flags.GetString("jobs")
		var jobsFilter []string
		if len(jobsStr) > 0 {
			jobsFilter = strings.Split(strings.Trim(jobsStr, ""), ",")
		}
		buildFlag, _ := flags.GetBool("build")
		if buildFlag {
			err := build.Execute()
			if err != nil {
				fmt.Println(err)
			}
		}

		err := deploy.Execute(jobsFilter)
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringP("jobs", "", "", "comma seperated jobs")
	deployCmd.Flags().BoolP("build", "b", false, "build before deploy")
}
