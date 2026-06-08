// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"

	"maand/cat"

	"github.com/spf13/cobra"
)

var catHashesCmd = &cobra.Command{
	Use:   "hashes",
	Short: "Show allocation content hashes and rollout state",
	Long: `Show current_hash and previous_hash per allocation for deploy debugging.

Use --jobs and --workers to filter (comma-separated, same as maand cat allocations).
Use --active to show only allocations deploy would target (removed=0, disabled=0).

Rollout: removed, disabled (catalog flags), or new, restart, promoted, health_failed (hash state).`,
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		jobsStr, _ := flags.GetString("jobs")
		workersStr, _ := flags.GetString("workers")
		activeOnly, _ := flags.GetBool("active")

		if err := cat.Hashes(jobsStr, workersStr, activeOnly); err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	catCmd.AddCommand(catHashesCmd)
	catHashesCmd.Flags().String("jobs", "", "Comma-separated job names")
	catHashesCmd.Flags().String("workers", "", "Comma-separated worker IPs")
	catHashesCmd.Flags().Bool("active", false, "Show only active allocations (removed=0, disabled=0)")
}
