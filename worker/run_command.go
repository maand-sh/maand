// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package worker

import (
	"fmt"
	"strings"

	"maand/bucket"
)

// ExecuteCommand runs commands on a worker over SSH.
func ExecuteCommand(rt *bucket.Runtime, workerIP string, commands []string, env []string) error {
	workerIP = strings.TrimSpace(workerIP)
	if workerIP == "" {
		return remoteError("", fmt.Errorf("worker IP is required"))
	}

	script := bucket.BuildCommandScript(commands, env)
	if err := RunRemoteScript(rt, workerIP, strings.NewReader(script), false); err != nil {
		return remoteError(workerIP, err)
	}
	return nil
}

// ExecuteFileCommand runs an existing bash script (host path under the bucket) on a worker.
// Environment variables must be embedded in the script file.
func ExecuteFileCommand(rt *bucket.Runtime, workerIP string, hostScriptPath string, _ []string) error {
	workerIP = strings.TrimSpace(workerIP)
	if workerIP == "" {
		return remoteError("", fmt.Errorf("worker IP is required"))
	}

	if err := RunRemoteScriptFile(rt, workerIP, hostScriptPath, false); err != nil {
		return remoteError(workerIP, err)
	}
	return nil
}
