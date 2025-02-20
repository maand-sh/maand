// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"
)

var catCmd = &cobra.Command{
	Use:   "cat",
	Short: "Shows bucket information",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Usage()
	},
}

func init() {
	maandCmd.AddCommand(catCmd)
}
