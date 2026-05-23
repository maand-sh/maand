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
	Run: func(cmd *cobra.Command, args []string) {
		err := cat.KV()
		if err != nil {
			log.Fatalln(err)
		}
	},
}

var catKVGetCmd = &cobra.Command{
	Use:   "get <namespace> <key>",
	Short: "Get a key and its value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		err := cat.KVGet(args[0], args[1])
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	catCmd.AddCommand(catKVCmd)
	catKVCmd.AddCommand(catKVListCmd)
	catKVCmd.AddCommand(catKVGetCmd)

	// Keep `maand cat kv` as a shortcut for listing all entries.
	catKVCmd.Run = catKVListCmd.Run
}
