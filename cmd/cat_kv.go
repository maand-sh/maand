// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"
	"maand/cat"

	"github.com/spf13/cobra"
)

var catKVCmd = &cobra.Command{
	Use:   "kv",
	Short: "List or get key-value store entries",
}

var catKVListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all keys and values",
	Run:   runCatKVList,
}

var catKVGetCmd = &cobra.Command{
	Use:   "get <namespace> <key>",
	Short: "Get a key and its value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		reveal, _ := cmd.Flags().GetBool("reveal")
		err := cat.KVGet(args[0], args[1], reveal)
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func runCatKVList(cmd *cobra.Command, _ []string) {
	flags := cmd.Flags()
	jobsStr, _ := flags.GetString("jobs")
	activeOnly, _ := flags.GetBool("active")
	deletedOnly, _ := flags.GetBool("deleted")
	if err := cat.KV(jobsStr, activeOnly, deletedOnly); err != nil {
		log.Fatalln(err)
	}
}

func init() {
	catCmd.AddCommand(catKVCmd)
	catKVCmd.AddCommand(catKVListCmd)
	catKVCmd.AddCommand(catKVGetCmd)
	catKVCmd.PersistentFlags().String("jobs", "", "Comma-separated job names (all KV namespaces accessible to the job)")
	catKVCmd.PersistentFlags().Bool("active", false, "Show only active keys (deleted=0)")
	catKVCmd.PersistentFlags().Bool("deleted", false, "Show only deleted keys (deleted=1)")
	catKVGetCmd.Flags().Bool("reveal", false, "Decrypt and show secrets/job values (requires secrets/kv.key)")

	// Keep `maand cat kv` as a shortcut for listing all entries.
	catKVCmd.Run = runCatKVList
}
