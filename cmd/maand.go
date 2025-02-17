package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var maandCmd = &cobra.Command{
	Use:   "maand",
	Short: "Maand is a agent less workload orchestrator",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Usage()
	},
}

func Execute() {
	if err := maandCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
