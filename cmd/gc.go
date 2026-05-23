// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"
	"maand/gc"

	"github.com/spf13/cobra"
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Cleanup unused objects in the bucket",
	Long:  "Removes soft-deleted allocations from maand.db, purges KV references for removed allocations, deletes worker data/logs/bin for removed allocations, and purges old key_value history.",
	Run: func(cmd *cobra.Command, args []string) {
		retainDays, _ := cmd.Flags().GetInt("retain-days")
		if err := gc.Execute(retainDays); err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(gcCmd)
	gcCmd.Flags().Int("retain-days", 0, "Days to retain deleted KV rows before purge (0 = aggressive)")
}
