// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"maand/bucket"
	"os"
	"path"
	"strings"
)

//go:embed Makefile
var makefile []byte

var jobCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		job := args[0]
		selectorComma, _ := flags.GetString("selectors")
		jobDir := path.Join(bucket.WorkspaceLocation, "jobs", job)
		if _, err := os.Stat(jobDir); os.IsNotExist(err) {
			var selectors = make([]string, 0)
			if len(selectorComma) > 0 {
				selectors = strings.Split(selectorComma, ",")
			}
			manifest := struct {
				Version   string   `json:"version"`
				Selectors []string `json:"selectors"`
			}{
				Version:   "1.0",
				Selectors: selectors,
			}
			manifestContent, _ := json.MarshalIndent(manifest, "", "    ")
			_ = os.MkdirAll(jobDir, 0755)
			_ = os.WriteFile(path.Join(jobDir, "manifest.json"), manifestContent, 0644)
			_ = os.WriteFile(path.Join(jobDir, "Makefile"), makefile, 0644)
		} else {
			fmt.Printf("job directory already exists: %s\n", job)
		}
	},
}

func init() {
	jobCmd.AddCommand(jobCreateCmd)
	jobCreateCmd.Flags().StringP("selectors", "s", "", "comma seperated selectors")
	// TODO: other manifest input for ease access.
}
