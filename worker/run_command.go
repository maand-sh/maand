// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package worker
package worker

import (
	"fmt"
	"os"
	"path"

	"maand/bucket"
)

func ExecuteCommand(dockerClient *bucket.DockerClient, workerIP string, commands []string, env []string) error {
	commandScriptFileName, err := bucket.GenerateCommandScript(commands, env)
	if err != nil {
		return err
	}

	commandScriptFilePath := path.Join(bucket.TempLocation, commandScriptFileName)
	defer func() {
		_ = os.Remove(commandScriptFilePath)
	}()

	return ExecuteFileCommand(dockerClient, workerIP, commandScriptFilePath, env)
}

func ExecuteFileCommand(dockerClient *bucket.DockerClient, workerIP string, commandScriptFileName string, env []string) error {
	conf, err := bucket.GetMaandConf()
	if err != nil {
		return err
	}

	user := conf.SSHUser
	keyFilePath := path.Join(bucket.SecretLocation, conf.SSHKeyFile)
	useSudo := conf.UseSUDO

	sh := "bash"
	if useSudo {
		sh = "sudo bash"
	}

	commandScriptBucketFilePath := path.Join(commandScriptFileName)

	sshCmd := fmt.Sprintf(
		`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o BatchMode=yes -o ConnectTimeout=10 -i %s %s@%s 'timeout 300 %s' < %s`,
		keyFilePath, user, workerIP, sh, commandScriptBucketFilePath)

	err = dockerClient.Exec(workerIP, []string{sshCmd}, nil, true)
	if err != nil {
		return err
	}

	return nil
}
