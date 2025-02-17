package cmd

import (
	"github.com/spf13/cobra"
	"maand/cat"
)

var catWorkersCmd = &cobra.Command{
	Use:   "workers",
	Short: "Shows available workers",
	Run: func(cmd *cobra.Command, args []string) {
		cat.Workers()
	},
}

func init() {
	catCmd.AddCommand(catWorkersCmd)
}
