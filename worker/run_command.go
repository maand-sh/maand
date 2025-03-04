// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package worker

import (
	"fmt"
	"maand/bucket"
	"maand/utils"
	"os"
	"path"
	"path/filepath"
)

func ExecuteCommand(workerIP string, commands []string, env []string) error {
	conf, err := utils.GetMaandConf()
	if err != nil {
		return err
	}

	user := conf.SSHUser
	keyFilePath, _ := filepath.Abs(path.Join(bucket.SecretLocation, conf.SSHKeyFile))
	useSudo := conf.UseSUDO

	scriptPath, err := utils.GenerateScript(commands, env)
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(scriptPath)
	}()

	sh := "bash"
	if useSudo {
		sh = "sudo bash"
	}

	sshCmd := fmt.Sprintf(
		"ssh -q -o BatchMode=true -o ConnectTimeout=10 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i '%s' %s@%s 'timeout 300 %s' < %s",
		keyFilePath, user, workerIP, sh, scriptPath,
	)

	return utils.ExecuteShellCommand(sshCmd, workerIP)
}

func ExecuteFileCommand(workerIP string, scriptPath string, env []string) error {
	conf, err := utils.GetMaandConf()
	if err != nil {
		return err
	}

	user := conf.SSHUser
	keyFilePath, _ := filepath.Abs(path.Join(bucket.SecretLocation, conf.SSHKeyFile))
	useSudo := conf.UseSUDO

	sh := "bash"
	if useSudo {
		sh = "sudo bash"
	}

	sshCmd := fmt.Sprintf(
		"ssh -q -o BatchMode=true -o ConnectTimeout=10 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i %s %s@%s '%s' < %s",
		keyFilePath, user, workerIP, sh, scriptPath,
	)

	return utils.ExecuteShellCommand(sshCmd, workerIP)
}
