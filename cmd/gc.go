// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"maand/gc"
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Cleanup unused objects in the bucket",
	Run: func(cmd *cobra.Command, args []string) {
		err := gc.Collect()
		if err != nil {
			log.Println(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(gcCmd)
}
