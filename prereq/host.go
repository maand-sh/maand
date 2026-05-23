// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package prereq

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"maand/bucket"
)

// HostError reports missing tools on the maand CLI host.
type HostError struct {
	Missing []string
}

func (e *HostError) Error() string {
	return fmt.Sprintf("host missing prerequisites: %s", strings.Join(e.Missing, ", "))
}

func (e *HostError) Is(target error) bool {
	return target == bucket.ErrHostPrerequisites
}

var (
	localDeployTools     = []string{"bash", "ssh", "rsync", "python3"}
	localRunCommandTools = []string{"bash", "ssh"}
)

// CheckLocalDeploy verifies host tools required before deploy (ssh/rsync/python3, bun when needed).
func CheckLocalDeploy(needsBun bool) error {
	tools := append([]string{}, localDeployTools...)
	if needsBun {
		tools = append(tools, "bun")
	}
	return CheckLocal(tools...)
}

// CheckLocalRunCommand verifies host tools required before run_command.
func CheckLocalRunCommand() error {
	return CheckLocal(localRunCommandTools...)
}

// CheckLocal verifies each command exists on PATH or is an executable file path.
func CheckLocal(commands ...string) error {
	missing := make([]string, 0, len(commands))
	for _, command := range commands {
		if !localCommandAvailable(command) {
			missing = append(missing, command)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", bucket.ErrHostPrerequisites, strings.Join(missing, ", "))
}

func localCommandAvailable(command string) bool {
	if strings.Contains(command, "/") {
		info, err := os.Stat(command)
		return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
	}
	_, err := exec.LookPath(command)
	return err == nil
}
