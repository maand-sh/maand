// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"log"
	"maand/initialize"
	"os"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initializes the bucket",
	Run: func(cmd *cobra.Command, args []string) {
		err := initialize.Execute()
		if errors.Is(err, initialize.BucketAlreadyInitializedErr) {
			log.Fatalln(err)
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
