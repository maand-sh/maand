// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"maand/cat"
)

var catKVCmd = &cobra.Command{
	Use:   "kv",
	Short: "Shows available key and values",
	Run: func(cmd *cobra.Command, args []string) {
		err := cat.KV()
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	catCmd.AddCommand(catKVCmd)
}
