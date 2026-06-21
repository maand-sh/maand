// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"

	"maand/runbooks"

	"github.com/spf13/cobra"
)

var runbooksCmd = &cobra.Command{
	Use:   "runbooks",
	Short: "Serve job runbooks from the catalog",
}

var runbooksServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve _prometheus/runbooks markdown over HTTP",
	Run: func(cmd *cobra.Command, args []string) {
		addr, _ := cmd.Flags().GetString("addr")
		if err := runbooks.Serve(addr); err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(runbooksCmd)
	runbooksCmd.AddCommand(runbooksServeCmd)
	runbooksServeCmd.Flags().String("addr", ":8080", "Address to listen on")
}
