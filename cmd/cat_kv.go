package cmd

import (
	"github.com/spf13/cobra"
	"maand/cat"
)

var catKVCmd = &cobra.Command{
	Use:   "kv",
	Short: "Shows available key and values",
	Run: func(cmd *cobra.Command, args []string) {
		cat.KV()
	},
}

func init() {
	catCmd.AddCommand(catKVCmd)
}
