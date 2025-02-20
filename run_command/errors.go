// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package run_command

type RunCommandError struct {
	Err map[string]error
}

func (e *RunCommandError) Error() string {
	return "Run command failed"
}
