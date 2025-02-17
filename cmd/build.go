package cmd

import (
	"github.com/spf13/cobra"
	"maand/build"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build and plan object in the bucket",
	Run: func(cmd *cobra.Command, args []string) {
		build.Execute()
	},
}

func init() {
	maandCmd.AddCommand(buildCmd)
}
