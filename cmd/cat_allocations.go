package cmd

import (
	"github.com/spf13/cobra"
	"maand/cat"
)

var catAllocationsCmd = &cobra.Command{
	Use:   "allocations",
	Short: "Shows available allocations",
	Run: func(cmd *cobra.Command, args []string) {
		cat.Allocations()
	},
}

func init() {
	catCmd.AddCommand(catAllocationsCmd)
}
