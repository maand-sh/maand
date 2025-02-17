package cmd

import (
	"github.com/spf13/cobra"
	"maand/initialize"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initializes the bucket",
	Run: func(cmd *cobra.Command, args []string) {
		initialize.Execute()
	},
}

func init() {
	maandCmd.AddCommand(initCmd)
}
