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
)

func ExecuteCommand(dockerClient *bucket.DockerClient, workerIP string, commands []string, env []string) error {
	commandScriptFileName, err := utils.GenerateCommandScript(commands, env)
	if err != nil {
		return err
	}

	defer func() {
		_ = os.Remove(path.Join(bucket.TempLocation, commandScriptFileName))
	}()

	return ExecuteFileCommand(dockerClient, workerIP, commandScriptFileName, env)
}

func ExecuteFileCommand(dockerClient *bucket.DockerClient, workerIP string, commandScriptFileName string, env []string) error {
	conf, err := utils.GetMaandConf()
	if err != nil {
		return err
	}

	err = KeyScan(dockerClient, workerIP)
	if err != nil {
		return err
	}

	user := conf.SSHUser
	keyFilePath := path.Join("/bucket", "secrets", conf.SSHKeyFile)
	useSudo := conf.UseSUDO

	sh := "bash"
	if useSudo {
		sh = "sudo bash"
	}

	commandScriptBucketFilePath := path.Join("/bucket", "tmp", commandScriptFileName)

	sshCmd := fmt.Sprintf(
		`ssh -o BatchMode=true -o ConnectTimeout=10 -i %s %s@%s 'timeout 300 %s' < %s`,
		keyFilePath, user, workerIP, sh, commandScriptBucketFilePath)

	err = dockerClient.Exec(workerIP, []string{sshCmd}, nil, true)
	if err != nil {
		return err
	}

	return nil
}
