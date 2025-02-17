package cmd

import (
	"github.com/spf13/cobra"
	"maand/cat"
)

var catJobPortsCmd = &cobra.Command{
	Use:   "job_ports",
	Short: "Shows available job ports",
	Run: func(cmd *cobra.Command, args []string) {
		cat.JobPorts()
	},
}

func init() {
	catCmd.AddCommand(catJobPortsCmd)
}
