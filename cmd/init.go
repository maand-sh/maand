// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"

	"maand/initialize"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize or upgrade the bucket",
	Long: `Create a new maand bucket in the current directory, or upgrade an existing bucket.

The first run creates maand.db, workspace layout, secrets, and tmp staging directories.
Later runs apply schema migrations and refresh catalog views without changing bucket_id or CA.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := initialize.Execute(); err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(initCmd)
}
