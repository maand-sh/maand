package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"maand/build"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build and plan object in the bucket",
	Run: func(cmd *cobra.Command, args []string) {
		err := build.Execute()
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(buildCmd)
}
