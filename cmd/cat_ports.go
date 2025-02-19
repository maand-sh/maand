package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"maand/cat"
)

var catJobPortsCmd = &cobra.Command{
	Use:   "job_ports",
	Short: "Shows available job ports",
	Run: func(cmd *cobra.Command, args []string) {
		err := cat.JobPorts()
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	catCmd.AddCommand(catJobPortsCmd)
}
