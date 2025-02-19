package cmd

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"maand/initialize"
	"os"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initializes the bucket",
	Run: func(cmd *cobra.Command, args []string) {
		err := initialize.Execute()
		if errors.Is(err, initialize.BucketAlreadyInitializedErr) {
			fmt.Println(err)
			os.Exit(1)
		}
		if err != nil {
			fmt.Println("unable to initialize bucket", err)
			os.Exit(1)
		}
	},
}

func init() {
	maandCmd.AddCommand(initCmd)
}
