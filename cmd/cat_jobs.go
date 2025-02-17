package cmd

import (
	"github.com/spf13/cobra"
	"maand/cat"
)

var catJobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "Shows available jobs",
	Run: func(cmd *cobra.Command, args []string) {
		cat.Jobs()
	},
}

func init() {
	catCmd.AddCommand(catJobsCmd)
}
