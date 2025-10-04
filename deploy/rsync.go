// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"fmt"
	"maand/bucket"
	"maand/utils"
	"path"
	"strings"
)

func rsync(dockerClient *bucket.DockerClient, bucketID, workerIP string) error {
	conf, err := utils.GetMaandConf()
	if err != nil {
		return err
	}

	user := conf.SSHUser
	keyFilePath := path.Join("/bucket", "secrets", conf.SSHKeyFile)
	useSUDO := conf.UseSUDO

	rs := "rsync"

	remoteRS := "/usr/bin/rsync"
	if useSUDO {
		remoteRS = "sudo /usr/bin/rsync"
	}

	ruleFilePath := path.Join("/bucket", "tmp", "workers", fmt.Sprintf("%s.rsync", workerIP))
	workerDir := path.Join("/bucket", "tmp", "workers", workerIP)

	rsOptions := []string{
		"--timeout=30",
		"--inplace",
		"--whole-file",
		"--checksum",
		"--recursive",
		"--force",
		"--delete-after",
		"--delete",
		"--group",
		"--owner",
		"--executability",
		"--compress",
		"--verbose",
		"--exclude=jobs/*/bin",
		"--exclude=jobs/*/data",
		"--exclude=jobs/*/logs",
		"--exclude=jobs/*/_modules",
		fmt.Sprintf("--rsync-path='%s'", remoteRS),
		fmt.Sprintf("--filter='merge %s'", ruleFilePath),
		fmt.Sprintf("--rsh='ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o BatchMode=yes -o ConnectTimeout=10 -i %s'", keyFilePath),
		fmt.Sprintf("%s/", workerDir),
		fmt.Sprintf("%s@%s:/opt/worker/%s", user, workerIP, bucketID),
	}

	cmd := append([]string{rs}, rsOptions...)
	if err = dockerClient.Exec(workerIP, []string{strings.Join(cmd, " ")}, nil, true); err != nil {
		return err
	}

	return nil
}
