package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"maand/cat"
)

var catAllocationsCmd = &cobra.Command{
	Use:   "allocations",
	Short: "Shows available allocations",
	Run: func(cmd *cobra.Command, args []string) {
		err := cat.Allocations()
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	catCmd.AddCommand(catAllocationsCmd)
}
