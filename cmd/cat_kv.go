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
