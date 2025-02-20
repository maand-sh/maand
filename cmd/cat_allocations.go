// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"maand/cat"
)

var catAllocationsCmd = &cobra.Command{
	Use:   "allocations",
	Short: "Shows available allocations",
	Run: func(cmd *cobra.Command, args []string) {
		err := cat.Allocations()
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	catCmd.AddCommand(catAllocationsCmd)
}
