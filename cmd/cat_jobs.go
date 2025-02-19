package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"maand/cat"
)

var catJobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "Shows available jobs",
	Run: func(cmd *cobra.Command, args []string) {
		err := cat.Jobs()
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	catCmd.AddCommand(catJobsCmd)
}
