// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"

	"maand/bucket"
	"maand/data"

	"github.com/spf13/cobra"
)

func requireCurrentSchema(cmd *cobra.Command) error {
	if skipSchemaCheck(cmd) {
		return nil
	}
	if err := data.CheckSchemaVersion(); err != nil {
		if errors.Is(err, bucket.ErrNotInitialized) {
			return fmt.Errorf("%w: run maand init in the bucket directory", bucket.ErrNotInitialized)
		}
		return err
	}
	return nil
}

func skipSchemaCheck(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "init", "help", "completion":
		return true
	}
	// Bare `maand` (usage only; no subcommand).
	return cmd.Parent() == nil
}
