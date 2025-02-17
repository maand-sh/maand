package cmd

import (
	"github.com/spf13/cobra"
	"maand/cat"
)

var catJobCommandsCmd = &cobra.Command{
	Use:   "job_commands",
	Short: "Shows available job commands",
	Run: func(cmd *cobra.Command, args []string) {
		cat.JobCommands()
	},
}

func init() {
	catCmd.AddCommand(catJobCommandsCmd)
}
