package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"maand/cat"
)

var catWorkersCmd = &cobra.Command{
	Use:   "workers",
	Short: "Shows available workers",
	Run: func(cmd *cobra.Command, args []string) {
		err := cat.Workers()
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	catCmd.AddCommand(catWorkersCmd)
}
