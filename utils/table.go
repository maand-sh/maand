// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package utils

import (
	"io"
	"os"

	"maand/bucket"

	"github.com/jedib0t/go-pretty/v6/table"
)

func tableOutput() io.Writer {
	if bucket.QuietCLIOutput() {
		return io.Discard
	}
	return os.Stdout
}

// NewStdoutTable creates a rounded table writer printing to stdout.
func NewStdoutTable(header table.Row) table.Writer {
	writer := table.NewWriter()
	writer.SetOutputMirror(tableOutput())
	writer.AppendHeader(header)
	writer.SetStyle(table.StyleRounded)
	return writer
}

// GetTable is deprecated; use NewStdoutTable.
func GetTable(header table.Row) table.Writer {
	return NewStdoutTable(header)
}
