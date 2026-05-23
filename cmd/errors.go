// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import "fmt"

func formatCommandError(command string, err error) string {
	return fmt.Sprintf("maand %s failed: %v", command, err)
}
