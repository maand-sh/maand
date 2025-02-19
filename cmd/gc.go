package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"maand/gc"
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Cleanup unused objects in the bucket",
	Run: func(cmd *cobra.Command, args []string) {
		err := gc.Collect()
		if err != nil {
			log.Println(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(gcCmd)
}
