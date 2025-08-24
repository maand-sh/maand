// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"
	"log"
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
		log.Fatalln(err)
	}
}
